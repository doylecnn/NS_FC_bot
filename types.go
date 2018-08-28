package main

type tomlConfig struct {
	Telegram telegramConfig
	Misc     miscConfig
	Database databaseConfig
}

type telegramConfig struct {
	Token         string
	UpdateTimeout int
	Debug         bool
	GroupID       int64
}

type miscConfig struct {
	Proxy string
}

type databaseConfig struct {
	DBName string
}
