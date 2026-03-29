package model

type Document struct {
	ID      int    `json:"id"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}
