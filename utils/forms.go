package utils

import "encoding/json"

/*
Form shit
*/
type FormButton struct {
	Text string `json:"text"`
	Image FormImage `json:"image"`
}

type FormImage struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type CustomFormField struct {
	Type string `json:"type"`
	Text string `json:"text"`
	PlaceHolder string `json:"placeholder"`
	Default string `json:"default"`
}

func CreateSimpleForm(Title, Description string, Buttons ...FormButton) []byte {
	buttons, _ := json.Marshal(Buttons)
	return []byte(`{"type":"form","title":"` + Title + `","content":"` + Description + `","buttons":` + string(buttons) + "}")
}

func CreateCustomForm(Title string, Fields ...CustomFormField) []byte {
	inputs, _ := json.Marshal(Fields)
	return []byte(`{"type":"custom_form","title":"` + Title + `","content":` + string(inputs) + `}`)
}
