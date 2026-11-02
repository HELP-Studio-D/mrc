package main

import (
	"fmt"
	"log"

	"github.com/HELP-Studio-D/mrc"
	"github.com/HELP-Studio-D/mrc/middleware"
)

func main() {
	// Главный элемент приложения для управлением всем фреймворком
	server, err := mrc.NewWithOptions(
		mrc.WithBindIP("127.0.0.1"),
		mrc.WithPort(8080),
	)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// Объявление логгера запросов на сервер
	server.Use(middleware.Logger())

	// Регистрация публичного роута
	server.Handle("hello", func(c *mrc.Context) {
		// Вы можете использовать также c.JSON(status, mrc.H{...:...})
		_ = c.String(mrc.StatusOK, "World!")
	})

	// Запуск сервера
	if err := server.Run(); err != nil {
		log.Fatal("Server isn't running, error:" + err.Error())
	}
}
