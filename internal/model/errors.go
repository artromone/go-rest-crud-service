package model

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input")

	// Конкретный текст добавляется обёрткой в сервисе:
	//   fmt.Errorf("%w: title must be at least 3 characters", ErrInvalidInput)
	//
	// Глагол %w вставляет ошибку в цепочку, поэтому
	// errors.Is(err, ErrInvalidInput) найдёт её на любой глубине.
	// С %v цепочка оборвалась бы — остался бы просто текст.
)
