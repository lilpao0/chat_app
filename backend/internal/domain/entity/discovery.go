package entity

type UserSearchQuery struct {
	Text    string
	AfterID int64
	Limit   int
}

type UserSearchPage struct {
	Items       []Participant
	HasMore     bool
	NextAfterID *int64
}

type DirectConversation struct {
	ID          int64
	Participant Participant
	Created     bool
}
