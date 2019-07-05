package httpcell

type CASReq struct {
	Current []byte `json:"current"`
	Next    []byte `json:"next"`
}

type CASRes struct {
	Changed bool   `json:"changed"`
	Current []byte `json:"current"`
}
