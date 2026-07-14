package game

import "time"

type EventCode string

const (
	EventGravity     EventCode = "GRAVITY_FAIL"
	EventInversion   EventCode = "INVERSION"
	EventBureaucracy EventCode = "BUREAUCRACY"
	EventChaos       EventCode = "CHAOS"
)

type EventDefinition struct {
	Code     EventCode
	Name     string
	Duration time.Duration
	Chance   float64
	Message  string
}

var EventRegistry = []EventDefinition{
	{
		Code:     EventGravity,
		Name:     "Сбой гравитации",
		Duration: 15 * time.Second,
		Chance:   0.4,
		Message:  "Внимание! Гравитация дала сбой. Ловите яйцо!",
	},
	{
		Code:     EventInversion,
		Name:     "Инверсия реальности",
		Duration: 10 * time.Second,
		Chance:   0.3,
		Message:  "Реальность инвертирована. Ничему не верьте.",
	},
	{
		Code:     EventBureaucracy,
		Name:     "Бюрократия",
		Duration: 20 * time.Second,
		Chance:   0.3,
		Message:  "Введено обязательное согласование кликов. Заполните форму 10-Б.",
	},
	{
		Code:     EventChaos,
		Name:     "Хаос",
		Duration: 12 * time.Second,
		Chance:   0.25,
		Message:  "Полный хаос! Всё возможно!",
	},
}
