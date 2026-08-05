package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/artem/tasks/internal/model"
)

// TaskRepository объявлен ЗДЕСЬ, в пакете-потребителе, а не в пакете repo.
//
// Это идиома Go: интерфейс принадлежит тому, кто его использует.
// Причины две:
//  1. реализаций несколько (postgres-репозиторий и мок в тестах), и странно,
//     когда контракт лежит внутри одной из реализаций;
//  2. здесь перечисляются ровно те методы, которые нужны сервису, а не все,
//     что есть у репозитория.
type TaskRepository interface {
	Create(ctx context.Context, in model.CreateTaskRequest) (model.Task, error)
	GetByID(ctx context.Context, id int64) (model.Task, error)
	List(ctx context.Context, f model.ListFilter) ([]model.Task, error)
	Update(ctx context.Context, id int64, in model.UpdateTaskRequest) (model.Task, error)
	Delete(ctx context.Context, id int64) error
}

const (
	defaultLimit    = 20
	maxAllowedLimit = 100
)

type TaskService struct {
	repo     TaskRepository
	validate *validator.Validate
}

// New принимает интерфейс — чтобы в тестах подставить мок.
func New(repo TaskRepository) *TaskService {
	return &TaskService{
		// Валидатор создаётся один раз и переиспользуется: внутри он кеширует
		// разбор тегов через рефлексию. Создавать его на каждый запрос —
		// заметная и совершенно бессмысленная трата.
		validate: validator.New(validator.WithRequiredStructEnabled()),
		repo:     repo,
	}
}

func (s *TaskService) Create(ctx context.Context, in model.CreateTaskRequest) (model.Task, error) {
	in.Title = strings.TrimSpace(in.Title)

	if err := s.check(in); err != nil {
		return model.Task{}, err
	}
	return s.repo.Create(ctx, in)
}

func (s *TaskService) GetByID(ctx context.Context, id int64) (model.Task, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *TaskService) List(ctx context.Context, f model.ListFilter) ([]model.Task, error) {
	if f.Limit <= 0 {
		f.Limit = defaultLimit
	}
	if f.Limit > maxAllowedLimit {
		f.Limit = maxAllowedLimit
	}
	return s.repo.List(ctx, f)
}

func (s *TaskService) Update(ctx context.Context, id int64, in model.UpdateTaskRequest) (model.Task, error) {
	if in.Title != nil {
		trimmed := strings.TrimSpace(*in.Title)
		in.Title = &trimmed
	}
	if err := s.check(in); err != nil {
		return model.Task{}, err
	}
	return s.repo.Update(ctx, id, in)
}

func (s *TaskService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *TaskService) check(v any) error {
	err := s.validate.Struct(v)
	if err == nil {
		return nil
	}

	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) && len(verrs) > 0 {
		// Берём первую ошибку: клиенту достаточно одной внятной причины.
		// Если нужен весь список — здесь его и собирают в слайс.
		return fmt.Errorf("%w: %s", model.ErrInvalidInput, humanize(verrs[0]))
	}
	// Сюда попадаем, если валидатору дали не структуру — это баг в коде,
	// а не проблема данных.
	return fmt.Errorf("validate: %w", err)
}

// humanize превращает машинную ошибку валидатора в текст для человека.
//
// По умолчанию validator отдаёт что-то вроде
// "Key: 'CreateTaskRequest.Title' Error:Field validation for 'Title' failed
// on the 'min' tag" — показывать такое клиенту нельзя.
func humanize(fe validator.FieldError) string {
	field := strings.ToLower(fe.Field())
	switch fe.Tag() {
	case "required":
		return field + " is required"
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, fe.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, fe.Param())
	default:
		return field + " is invalid"
	}
}
