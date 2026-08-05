package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/artem/tasks/internal/model"
)

type TaskRepo struct {
	pool *pgxpool.Pool
}

func NewTaskRepo(pool *pgxpool.Pool) *TaskRepo {
	return &TaskRepo{pool: pool}
}

const taskColumns = `id, title, done, due_date, created_at`

func (r *TaskRepo) Create(ctx context.Context, in model.CreateTaskRequest) (model.Task, error) {
	// $1, $2 — параметры, а не форматирование строки. Драйвер отправляет
	// запрос и значения раздельно, поэтому Postgres физически не может
	// интерпретировать значение как SQL. Это и есть защита от инъекций.
	//
	// RETURNING — чтобы забрать сгенерированные id и created_at сразу без SELECT
	const q = `
		INSERT INTO tasks (title, due_date)
		VALUES ($1, $2)
		RETURNING ` + taskColumns

	var t model.Task
	err := r.pool.QueryRow(ctx, q, in.Title, in.DueDate).
		Scan(&t.ID, &t.Title, &t.Done, &t.DueDate, &t.CreatedAt)
	if err != nil {
		return model.Task{}, fmt.Errorf("create task: %w", err)
	}
	return t, nil
}

func (r *TaskRepo) GetByID(ctx context.Context, id int64) (model.Task, error) {
	const q = `SELECT ` + taskColumns + ` FROM tasks WHERE id = $1`

	var t model.Task
	err := r.pool.QueryRow(ctx, q, id).
		Scan(&t.ID, &t.Title, &t.Done, &t.DueDate, &t.CreatedAt)

	// ошибка драйвера превращается в доменную.
	//
	// errors.Is, а не ==: если кто-то по пути обернёт ошибку через %w,
	// сравнение на равенство молча перестанет работать, а errors.Is
	// развернёт всю цепочку Unwrap.
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Task{}, model.ErrNotFound
	}
	if err != nil {
		return model.Task{}, fmt.Errorf("get task %d: %w", id, err)
	}
	return t, nil
}

func (r *TaskRepo) List(ctx context.Context, f model.ListFilter) ([]model.Task, error) {
	const q = `
		SELECT ` + taskColumns + `
		FROM tasks
		WHERE ($1::bool IS NULL OR done = $1)
		ORDER BY created_at DESC, id DESC
		LIMIT $2`

	rows, err := r.pool.Query(ctx, q, f.Done, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	defer rows.Close()

	// Инициализируем пустым слайсом, а не оставляем nil: nil сериализуется
	// в JSON как null, а пустой слайс — как []. Клиенту удобнее [].
	tasks := make([]model.Task, 0)
	for rows.Next() {
		var t model.Task
		if err := rows.Scan(&t.ID, &t.Title, &t.Done, &t.DueDate, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}
	return tasks, nil
}

func (r *TaskRepo) Update(ctx context.Context, id int64, in model.UpdateTaskRequest) (model.Task, error) {
	const q = `
		UPDATE tasks
		SET title    = COALESCE($2, title),
		    done     = COALESCE($3, done),
		    due_date = COALESCE($4, due_date)
		WHERE id = $1
		RETURNING ` + taskColumns

	var t model.Task
	err := r.pool.QueryRow(ctx, q, id, in.Title, in.Done, in.DueDate).
		Scan(&t.ID, &t.Title, &t.Done, &t.DueDate, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		// UPDATE ... RETURNING ничего не вернул — значит строки с таким id нет.
		return model.Task{}, model.ErrNotFound
	}
	if err != nil {
		return model.Task{}, fmt.Errorf("update task %d: %w", id, err)
	}
	return t, nil
}

func (r *TaskRepo) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM tasks WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("delete task %d: %w", id, err)
	}
	// DELETE несуществующей строки — не ошибка на уровне SQL: запрос
	// отработал успешно, просто ничего не удалил.
	if tag.RowsAffected() == 0 {
		return model.ErrNotFound
	}
	return nil
}
