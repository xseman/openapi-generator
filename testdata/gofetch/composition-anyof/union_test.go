package compositionanyof

import (
	"encoding/json"
	"testing"
)

func TestAnyOf(t *testing.T) {
	var c ContactMethod
	if err := json.Unmarshal([]byte(`{"email":"a@b.c","phone":"1"}`), &c); err != nil {
		t.Fatal(err)
	}

	email, err := c.AsEmailContact()
	if err != nil || email.Email != "a@b.c" {
		t.Errorf("AsEmailContact = %+v, %v", email, err)
	}

	phone, err := c.AsPhoneContact()
	if err != nil || phone.Phone != "1" {
		t.Errorf("AsPhoneContact = %+v, %v", phone, err)
	}

	if err := c.FromPhoneContact(PhoneContact{Phone: "2"}); err != nil {
		t.Fatal(err)
	}

	if b, _ := json.Marshal(c); string(b) != `{"phone":"2"}` {
		t.Errorf("Marshal = %s", b)
	}
}
