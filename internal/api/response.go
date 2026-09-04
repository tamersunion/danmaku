package api

type result struct {
	Code int `json:"code"`
	Data any `json:"data"`
}

func success(data any) result { return result{Code: 0, Data: data} }

func failure() result { return result{Code: 1, Data: nil} }
