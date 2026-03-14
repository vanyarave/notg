package models

type Message struct {
	Type string `json:"type"`
	Room string `json:"room"`
	User string `json:"user"`
	Text string `json:"text"`
}
