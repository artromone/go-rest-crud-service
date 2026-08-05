package model

import "time"

type Task struct {
	ID        int64      `json:"id"`
	Title     string     `json:"title"`
	Done      bool       `json:"done"`
	DueDate   *time.Time `json:"due_date,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// CreateTaskRequest ...
// Теги validate читает go-playground/validator в сервисном слое.
// min/max для строк он считает в рунах, а не в байтах — то есть кириллица
// посчитается правильно.
type CreateTaskRequest struct {
	Title   string     `json:"title" validate:"required,min=3,max=100"`
	DueDate *time.Time `json:"due_date"`
}

// UpdateTaskRequest ...
// Все поля — указатели, и это принципиально. С обычным bool на запросе
// {"title":"новое"} поле Done будет false (zero value), и мы не отличим
// "клиент прислал false" от "клиент про поле не написал".
//
// nil       — поле не прислали, не трогаем;
// указатель — прислали, применяем (даже если внутри false или "").
type UpdateTaskRequest struct {
	Title   *string    `json:"title" validate:"omitempty,min=3,max=100"`
	Done    *bool      `json:"done"`
	DueDate *time.Time `json:"due_date"`
}

type ListFilter struct {
	Done  *bool // nil — без фильтра по статусу
	Limit int
}
