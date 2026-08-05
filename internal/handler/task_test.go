package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/artem/tasks/internal/model"
)

type mockService struct {
	createFn  func(ctx context.Context, in model.CreateTaskRequest) (model.Task, error)
	getByIDFn func(ctx context.Context, id int64) (model.Task, error)
	listFn    func(ctx context.Context, f model.ListFilter) ([]model.Task, error)
	updateFn  func(ctx context.Context, id int64, in model.UpdateTaskRequest) (model.Task, error)
	deleteFn  func(ctx context.Context, id int64) error
}

func (m *mockService) Create(ctx context.Context, in model.CreateTaskRequest) (model.Task, error) {
	return m.createFn(ctx, in)
}

func (m *mockService) GetByID(ctx context.Context, id int64) (model.Task, error) {
	return m.getByIDFn(ctx, id)
}

func (m *mockService) List(ctx context.Context, f model.ListFilter) ([]model.Task, error) {
	return m.listFn(ctx, f)
}

func (m *mockService) Update(ctx context.Context, id int64, in model.UpdateTaskRequest) (model.Task, error) {
	return m.updateFn(ctx, id, in)
}
func (m *mockService) Delete(ctx context.Context, id int64) error { return m.deleteFn(ctx, id) }

// stubPinger — заглушка для healthz, чтобы собрать роутер без базы.
type stubPinger struct{ err error }

func (s stubPinger) Ping(context.Context) error { return s.err }

// newTestServer собирает полный роутер с моком вместо сервиса.
//
// Тестируем через роутер, а не вызовом хендлера напрямую: так в покрытие
// попадает и маршрутизация, и middleware — то есть проверяется ровно то,
// что увидит настоящий клиент.
func newTestServer(svc TaskService) http.Handler {
	return NewRouter(NewTaskHandler(svc), stubPinger{})
}

// do выполняет запрос без единого сокета.
//
// httptest.NewRecorder — реализация http.ResponseWriter, которая просто
// пишет в буфер. Никаких портов, никакого Listen: тест бежит миллисекунды.
// Возможно это ровно потому, что ResponseWriter — интерфейс.
func do(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, target, nil)
	} else {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// ─── Тесты ───────────────────────────────────────────────────────────────

func TestCreate_Created(t *testing.T) {
	svc := &mockService{
		createFn: func(_ context.Context, in model.CreateTaskRequest) (model.Task, error) {
			require.Equal(t, "изучить net/http", in.Title)
			return model.Task{ID: 1, Title: in.Title}, nil
		},
	}

	rec := do(t, newTestServer(svc), http.MethodPost, "/api/v1/tasks",
		`{"title":"изучить net/http"}`)

	require.Equal(t, http.StatusCreated, rec.Code)

	var got model.Task
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, int64(1), got.ID)
}

func TestCreate_MalformedJSON(t *testing.T) {
	// Сервис не должен быть вызван вообще — структурная валидация
	// отсекает запрос в хендлере.
	svc := &mockService{
		createFn: func(context.Context, model.CreateTaskRequest) (model.Task, error) {
			t.Fatal("service must not be called on malformed JSON")
			return model.Task{}, nil
		},
	}

	rec := do(t, newTestServer(svc), http.MethodPost, "/api/v1/tasks", `{"title":`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreate_ValidationError(t *testing.T) {
	// Бизнес-валидация живёт в сервисе, а хендлер обязан превратить
	// ErrInvalidInput именно в 400, а не в 500.
	svc := &mockService{
		createFn: func(context.Context, model.CreateTaskRequest) (model.Task, error) {
			return model.Task{}, fmt.Errorf("%s: %w", "title must be at least 3 characters", model.ErrInvalidInput)
		},
	}

	rec := do(t, newTestServer(svc), http.MethodPost, "/api/v1/tasks", `{"title":"ab"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "at least 3 characters")
}

func TestGetByID_NotFound(t *testing.T) {
	svc := &mockService{
		getByIDFn: func(context.Context, int64) (model.Task, error) {
			return model.Task{}, model.ErrNotFound
		},
	}

	rec := do(t, newTestServer(svc), http.MethodGet, "/api/v1/tasks/42", "")

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetByID_BadID(t *testing.T) {
	svc := &mockService{
		getByIDFn: func(context.Context, int64) (model.Task, error) {
			t.Fatal("service must not be called with non-numeric id")
			return model.Task{}, nil
		},
	}

	rec := do(t, newTestServer(svc), http.MethodGet, "/api/v1/tasks/abc", "")

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestList_EmptyIsArrayNotNull(t *testing.T) {
	svc := &mockService{
		listFn: func(context.Context, model.ListFilter) ([]model.Task, error) {
			return []model.Task{}, nil
		},
	}

	rec := do(t, newTestServer(svc), http.MethodGet, "/api/v1/tasks", "")

	require.Equal(t, http.StatusOK, rec.Code)
	// Пустой список должен сериализоваться в [], а не null —
	// иначе фронт получит null и упадёт на .map().
	require.Equal(t, "[]", strings.TrimSpace(rec.Body.String()))
}

func TestList_DoneFilterParsed(t *testing.T) {
	svc := &mockService{
		listFn: func(_ context.Context, f model.ListFilter) ([]model.Task, error) {
			// Именно здесь видно, зачем указатель: "?done=false" должен
			// доехать как &false, а не потеряться как zero value.
			require.NotNil(t, f.Done)
			require.False(t, *f.Done)
			require.Equal(t, 5, f.Limit)
			return nil, nil
		},
	}

	rec := do(t, newTestServer(svc), http.MethodGet, "/api/v1/tasks?done=false&limit=5", "")

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestList_NoFilter(t *testing.T) {
	svc := &mockService{
		listFn: func(_ context.Context, f model.ListFilter) ([]model.Task, error) {
			require.Nil(t, f.Done) // фильтр не задан — не &false, а nil
			return nil, nil
		},
	}

	rec := do(t, newTestServer(svc), http.MethodGet, "/api/v1/tasks", "")

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestUpdate_PartialKeepsOtherFields(t *testing.T) {
	svc := &mockService{
		updateFn: func(_ context.Context, id int64, in model.UpdateTaskRequest) (model.Task, error) {
			require.Equal(t, int64(7), id)
			require.NotNil(t, in.Title)
			// Done не прислали — значит nil, и репозиторий его не тронет.
			// С обычным bool здесь был бы false, и задача молча
			// "развыполнилась" бы.
			require.Nil(t, in.Done)
			return model.Task{ID: id, Title: *in.Title, Done: true}, nil
		},
	}

	rec := do(t, newTestServer(svc), http.MethodPatch, "/api/v1/tasks/7",
		`{"title":"новое название"}`)

	require.Equal(t, http.StatusOK, rec.Code)

	var got model.Task
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.True(t, got.Done, "done must survive a partial update")
}

func TestUpdate_ExplicitFalseIsApplied(t *testing.T) {
	svc := &mockService{
		updateFn: func(_ context.Context, _ int64, in model.UpdateTaskRequest) (model.Task, error) {
			// А вот здесь false прислали явно — и он должен доехать.
			require.NotNil(t, in.Done)
			require.False(t, *in.Done)
			return model.Task{ID: 7, Done: false}, nil
		},
	}

	rec := do(t, newTestServer(svc), http.MethodPatch, "/api/v1/tasks/7", `{"done":false}`)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestDelete_NoContent(t *testing.T) {
	svc := &mockService{
		deleteFn: func(context.Context, int64) error { return nil },
	}

	rec := do(t, newTestServer(svc), http.MethodDelete, "/api/v1/tasks/1", "")

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Body.String(), "204 must have no body")
}

func TestDelete_NotFound(t *testing.T) {
	// Удаление несуществующей записи — это 404, а не 204.
	// В репозитории за это отвечает проверка RowsAffected() == 0.
	svc := &mockService{
		deleteFn: func(context.Context, int64) error { return model.ErrNotFound },
	}

	rec := do(t, newTestServer(svc), http.MethodDelete, "/api/v1/tasks/999", "")

	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestMethodNotAllowed(t *testing.T) {
	// ServeMux сам отдаёт 405, если путь совпал, а метод — нет.
	// До Go 1.22 это писали руками.
	rec := do(t, newTestServer(&mockService{}), http.MethodPut, "/api/v1/tasks/1", "{}")

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}

func TestPanicRecovered(t *testing.T) {
	// Паника в хендлере не должна ронять процесс — Recovery превращает
	// её в 500 для одного клиента.
	svc := &mockService{
		listFn: func(context.Context, model.ListFilter) ([]model.Task, error) {
			panic("boom")
		},
	}

	rec := do(t, newTestServer(svc), http.MethodGet, "/api/v1/tasks", "")

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestRequestIDPropagated(t *testing.T) {
	svc := &mockService{
		listFn: func(context.Context, model.ListFilter) ([]model.Task, error) {
			return nil, nil
		},
	}

	rec := do(t, newTestServer(svc), http.MethodGet, "/api/v1/tasks", "")

	require.NotEmpty(t, rec.Header().Get("X-Request-Id"))
}
