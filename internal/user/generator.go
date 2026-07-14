package user

import (
	"fmt"
	"math/rand"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var adjectives = []string{
	"Грустный", "Токсичный", "Анонимный", "Синий", "Голодный",
}

var nouns = []string{
	"Пельмень", "Кактус", "Дедлайн", "Бэкендер", "Соседский Кот",
}

func GenerateName() string {
	adj := adjectives[rand.Intn(len(adjectives))]
	noun := nouns[rand.Intn(len(nouns))]

	return fmt.Sprintf("%s %s", adj, noun)
}
