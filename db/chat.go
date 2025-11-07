package db

import (
	"fmt"
	"database/sql"
)

func GetChatById(db *sql.DB, chat int64) (int16, string, error) {
	query := `SELECT rate, working_mode FROM chat WHERE chat = $1`
	var rate int16
	var mode string
	err := db.QueryRow(query, chat).Scan(&rate, &mode); 
	if err == sql.ErrNoRows {
		err = InitDefaultSettings(db, chat);
		if err != nil {
			return 0, "", fmt.Errorf("can't initialize default settings: %s", err.Error())
		}
		return GetChatById(db, chat)
	}
	if err != nil {
		return 0, "", err
	}
	return rate, mode, nil
}

func ApplyChatSettingsById(db *sql.DB, chat int64, rate int16, mode string) error {
	if mode != "on" && mode != "learning" && mode != "messaging" && mode != "off" {
		return fmt.Errorf("cannot apply working mode: %s does not allowed", mode)
	}
	err := InitDefaultSettings(db, chat)
	if err != nil { return err; }
	query := `UPDATE chat SET rate = $1, working_mode = $2 WHERE chat = $3`
	_, err = db.Exec(query, rate, mode, chat)
	return err
}

func InitDefaultSettings(db *sql.DB, chat int64) error {
	query := `
		INSERT INTO chat (chat) VALUES ($1) ON CONFLICT (chat) DO NOTHING;
	`
	if _, err := db.Exec(query, chat); err != nil { return err }
	return nil
}

func ResetToDefaultSettings(db *sql.DB, chat int64) error {
	query := `
		DELETE FROM chat WHERE chat = $1
	`
	if _, err := db.Exec(query, chat); err != nil { return err }
	return InitDefaultSettings(db, chat)
}
