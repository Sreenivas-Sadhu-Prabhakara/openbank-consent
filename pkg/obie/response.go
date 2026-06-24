package obie

// Links is the OBIE Links object attached to every response. Only Self is
// mandatory; the pagination links are populated when a collection spans
// multiple pages.
type Links struct {
	Self  string `json:"Self"`
	First string `json:"First,omitempty"`
	Prev  string `json:"Prev,omitempty"`
	Next  string `json:"Next,omitempty"`
	Last  string `json:"Last,omitempty"`
}

// Meta is the OBIE Meta object. TotalPages is reported for collection
// responses; the timestamps bound the data returned for transaction queries.
type Meta struct {
	TotalPages         int    `json:"TotalPages,omitempty"`
	FirstAvailableDate string `json:"FirstAvailableDateTime,omitempty"`
	LastAvailableDate  string `json:"LastAvailableDateTime,omitempty"`
}

// Response is the canonical OBIE envelope: a typed Data payload alongside the
// Links and Meta objects. Services construct it with NewResponse so the Self
// link and Meta are always present.
type Response struct {
	Data  any   `json:"Data"`
	Links Links `json:"Links"`
	Meta  Meta  `json:"Meta"`
}

// NewResponse wraps a data payload with a Self link and a single-page Meta.
// Use WithPagination on the result to describe a multi-page collection.
func NewResponse(self string, data any) Response {
	return Response{
		Data:  data,
		Links: Links{Self: self},
		Meta:  Meta{TotalPages: 1},
	}
}
