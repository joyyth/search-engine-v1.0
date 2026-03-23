package model

type Document struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}