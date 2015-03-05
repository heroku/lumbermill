package main

import "testing"

func TestParseCreds(t *testing.T) {
	var creds map[string]string
	var err error

	creds, err = parseCreds("user:pass")
	if err == nil {
		if val, ok := creds["user"]; !ok || val != "pass" {
			t.Fatalf("valid credential user:pass failed.")
		}
	} else {
		t.Fatalf("valid credentials returned an error: ", err)
	}

	// Multiple credentials
	creds, err = parseCreds("user1:pass1|user:pass")
	if err == nil {
		if val, ok := creds["user"]; !ok || val != "pass" {
			t.Fatalf("valid credential user:pass failed.")
		}
		if val, ok := creds["user1"]; !ok || val != "pass1" {
			t.Fatalf("valid credential user:pass failed.")
		}
	} else {
		t.Fatalf("valid credentials returned an error: ", err)
	}

	creds, err = parseCreds("user1")
	if err == nil {
		t.Fatalf("invalid credential passed parsing. (%+v)", creds)
	}
}
