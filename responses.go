package main

type UserResponse struct {
	BenisHistory [][]int32 `json:"benisHistory,omitempty"`
}

type UserSuggestResponse struct {
	Names []string `json:"names"`
}
