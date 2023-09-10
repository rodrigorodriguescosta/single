package auth

import "github.com/kissprojects/single/comps/go/str"

// inputParser perform some parsers over the model
func inputParser(model *Model) {
	model.Name = str.UpperNoSpaceNoAccent(model.Name)
	model.Email = str.LowerNoSpaceNoAccent(model.Email)
}
