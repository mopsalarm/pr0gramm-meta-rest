package main

type UserResponse struct {
  BenisHistory [][]int32 `json:"benisHistory,omitempty"`
}

type UserSuggestResponse struct {
  Names []string `json:"names"`
}

type SizeInfo struct {
  Id     int64 `json:"id"`
  Width  int32 `json:"width"`
  Height int32 `json:"height"`
}

type PreviewInfo struct {
  Id     int64 `json:"id"`
  Width  int32 `json:"width"`
  Height int32 `json:"height"`
  Pixels string `json:"pixels"`
}

type InfoResponse struct {
  Duration float64 `json:"duration"`
  Reposts  []int64 `json:"reposts"`
  Sizes    []SizeInfo `json:"sizes"`
  Previews []PreviewInfo `json:"previews"`
}

