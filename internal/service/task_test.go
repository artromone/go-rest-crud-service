package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/artem/tasks/internal/model"
)

type mockRepo struct {
	createFn func(ctx context.Context, in model.CreateTaskRequest) (model.Task, error)
	listFn   func(ctx context.Context, f model.ListFilter) ([]model.Task, error)
	updateFn func(ctx context.Context, id int64, in model.UpdateTaskRequest) (model.Task, error)
}

func (m *mockRepo) Create(ctx context.Context, in model.CreateTaskRequest) (model.Task, error) {
	return m.createFn(ctx, in)
}

func (m *mockRepo) GetByID(context.Context, int64) (model.Task, error) {
	return model.Task{}, nil
}

func (m *mockRepo) List(ctx context.Context, f model.ListFilter) ([]model.Task, error) {
	return m.listFn(ctx, f)
}

func (m *mockRepo) Update(ctx context.Context, id int64, in model.UpdateTaskRequest) (model.Task, error) {
	return m.updateFn(ctx, id, in)
}
func (m *mockRepo) Delete(context.Context, int64) error { return nil }

func TestCreate_TitleTooShort(t *testing.T) {
	repo := &mockRepo{
		createFn: func(context.Context, model.CreateTaskRequest) (model.Task, error) {
			t.Fatal("repo must not be called on invalid input")
			return model.Task{}, nil
		},
	}

	_, err := New(repo).Create(context.Background(), model.CreateTaskRequest{Title: "ab"})

	// errors.Is, а не сравнение строк: ошибка обёрнута, но Unwrap
	// доводит проверку до ErrInvalidInput.
	require.True(t, errors.Is(err, model.ErrInvalidInput))
}

func TestCreate_TitleTrimmed(t *testing.T) {
	// "  ab  " после TrimSpace — это "ab", то есть 2 символа.
	// Без Trim пробелы прошли бы валидацию.
	repo := &mockRepo{
		createFn: func(context.Context, model.CreateTaskRequest) (model.Task, error) {
			t.Fatal("repo must not be called")
			return model.Task{}, nil
		},
	}

	_, err := New(repo).Create(context.Background(), model.CreateTaskRequest{Title: "  ab  "})

	require.True(t, errors.Is(err, model.ErrInvalidInput))
}

func TestCreate_CyrillicLengthCountedInRunes(t *testing.T) {
	// 100 кириллических символов — это 200 байт. С len() эта задача
	// не прошла бы валидацию, хотя по правилу она валидна.
	title := strings.Repeat("я", 100)

	repo := &mockRepo{
		createFn: func(_ context.Context, in model.CreateTaskRequest) (model.Task, error) {
			return model.Task{Title: in.Title}, nil
		},
	}

	_, err := New(repo).Create(context.Background(), model.CreateTaskRequest{Title: title})

	require.NoError(t, err)
}

func TestList_DefaultLimit(t *testing.T) {
	repo := &mockRepo{
		listFn: func(_ context.Context, f model.ListFilter) ([]model.Task, error) {
			require.Equal(t, defaultLimit, f.Limit)
			return nil, nil
		},
	}

	_, err := New(repo).List(context.Background(), model.ListFilter{})
	require.NoError(t, err)
}

func TestList_LimitCapped(t *testing.T) {
	// Клиент не должен уметь выгрести миллион строк одним запросом.
	repo := &mockRepo{
		listFn: func(_ context.Context, f model.ListFilter) ([]model.Task, error) {
			require.Equal(t, maxAllowedLimit, f.Limit)
			return nil, nil
		},
	}

	_, err := New(repo).List(context.Background(), model.ListFilter{Limit: 1_000_000})
	require.NoError(t, err)
}

func TestUpdate_NilTitleNotValidated(t *testing.T) {
	// Title не прислали — валидировать нечего, до репозитория доходим.
	repo := &mockRepo{
		updateFn: func(_ context.Context, id int64, in model.UpdateTaskRequest) (model.Task, error) {
			require.Nil(t, in.Title)
			return model.Task{ID: id}, nil
		},
	}

	done := true
	_, err := New(repo).Update(context.Background(), 1, model.UpdateTaskRequest{Done: &done})
	require.NoError(t, err)
}

func TestUpdate_EmptyTitleRejected(t *testing.T) {
	// А вот пустую строку прислали явно — и это невалидно.
	repo := &mockRepo{
		updateFn: func(context.Context, int64, model.UpdateTaskRequest) (model.Task, error) {
			t.Fatal("repo must not be called")
			return model.Task{}, nil
		},
	}

	empty := ""
	_, err := New(repo).Update(context.Background(), 1, model.UpdateTaskRequest{Title: &empty})
	require.True(t, errors.Is(err, model.ErrInvalidInput))
}
