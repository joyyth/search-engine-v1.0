package model

type Document struct {
	ID      int    `json:"id"`
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// Posting is one inverted-index entry: a document id and term frequency in that doc.
type Posting struct {
	DocID int     `json:"doc_id"`
	TF    float64 `json:"tf"`
}
