package core

import "testing"

func TestEmbeds(t *testing.T) {
	emb := SuccessEmbed("Success", "desc")
	if emb.Title != "Success" || emb.Description != "desc" {
		t.Fatal("SuccessEmbed")
	}
	emb = ErrorEmbed("Error", "desc")
	if emb.Title != "Error" || emb.Description != "desc" {
		t.Fatal("ErrorEmbed")
	}
	emb = InfoEmbed("Info", "desc")
	if emb.Title != "Info" || emb.Description != "desc" {
		t.Fatal("InfoEmbed")
	}
	emb = WarningEmbed("Warning", "desc")
	if emb.Title != "Warning" || emb.Description != "desc" {
		t.Fatal("WarningEmbed")
	}
}
