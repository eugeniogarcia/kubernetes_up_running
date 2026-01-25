package debugprobe

// ProbeStatus is returned from a GET to this API endpoing
type ProbeStatus struct {
	ProbePath string               `json:"probePath"`
	FailNext  int                  `json:"failNext"`
	History   []ProbeStatusHistory `json:"history"`
}

// ProbeStatusHistory is a record of a probe call
type ProbeStatusHistory struct {
	ID      int    `json:"id"`
	When    string `json:"when"`
	RelWhen string `json:"relWhen"`
	Code    int    `json:"code"`
}
