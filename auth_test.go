package main

import (
	"bytes"
	"encoding/json"
	. "github.com/smartystreets/goconvey/convey"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUser(t *testing.T) {
	Convey("Methods work as expected", t, func() {
		user := new(User)
		Convey("Setting and verify password works correctly with hashes", func() {
			user.SetPassword([]byte("hello123"))
			So(user.Password, ShouldStartWith, "$")

			So(user.VerifyPassword([]byte("hello123")), ShouldBeNil)
			So(user.VerifyPassword([]byte("hello12")), ShouldNotBeNil)
		})

		Convey("Invalid hash returns the correct error code", func() {
			user.Password = "I DON'T WORK"
			So(user.VerifyPassword([]byte("hello123")).Error(), ShouldContainSubstring, "hashedSecret too short")
		})
	})
}

func TestJWTGeneration(t *testing.T) {
	Convey("test basic claim creation", t, func() {
		ts, err := newJWT("hello test")
		So(ts, ShouldNotBeNil)
		So(err, ShouldBeNil)
	})
}

func TestLogin(t *testing.T) {
	// setup the fake db
	db, err := openDb("./tmp/test.db")
	if err != nil {
		panic(err)
	}
	ENV.DB = db

	user := &User{
		Email: "login@test.case",
	}
	user.SetPassword([]byte("testing123"))
	ENV.DB.Save(user)

	Convey("Valid request works as expected", t, func() {
		lp := &LoginPayload{
			Email:    "login@test.case",
			Password: "testing123",
		}
		body, _ := json.Marshal(lp)

		req := httptest.NewRequest("POST", "/api/login/", bytes.NewBuffer(body))
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(Login)

		req.Header.Add("Content-Type", "application/json")
		handler.ServeHTTP(rr, req)

		So(rr.Code, ShouldEqual, http.StatusOK)
		rr.Result().Body.Read(body)
		So(string(body), ShouldContainSubstring, `"token":`)
	})

	Convey("Invalid credentials return error", t, func() {
		Convey("Incorrect username provides 404", func() {
			lp := &LoginPayload{
				Email:    "login-no@test.case",
				Password: "testing123",
			}
			body, _ := json.Marshal(lp)

			req := httptest.NewRequest("POST", "/api/login/", bytes.NewBuffer(body))
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(Login)

			req.Header.Add("Content-Type", "application/json")
			handler.ServeHTTP(rr, req)

			So(rr.Code, ShouldEqual, http.StatusNotFound)
		})

		Convey("Incorrect password provides 403", func() {
			lp := &LoginPayload{
				Email:    "login@test.case",
				Password: "testing12",
			}
			body, _ := json.Marshal(lp)

			req := httptest.NewRequest("POST", "/api/login/", bytes.NewBuffer(body))
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(Login)

			req.Header.Add("Content-Type", "application/json")
			handler.ServeHTTP(rr, req)

			So(rr.Code, ShouldEqual, http.StatusForbidden)
		})
	})

	Convey("Server errors render nicely", t, func() {
		SkipConvey("Password is broken", func() {
			pass := "I DON'T WORK"
			ENV.DB.One("Email", user.Email, &user)
			user.Password = pass
			//ENV.DB.Save(user)

			ENV.DB.Find("Email", user.Email, &user)
			So(user.Password, ShouldEqual, pass)

			lp := &LoginPayload{
				Email:    "login@test.case",
				Password: "testing12",
			}
			body, _ := json.Marshal(lp)

			req := httptest.NewRequest("POST", "/api/login/", bytes.NewBuffer(body))
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(Login)

			req.Header.Add("Content-Type", "application/json")
			handler.ServeHTTP(rr, req)

			So(rr.Code, ShouldEqual, http.StatusInternalServerError)
		})
	})
}
