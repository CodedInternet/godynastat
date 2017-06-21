package main

import (
	"fmt"
	. "github.com/CodedInternet/godynastat/onboard"
	"github.com/abiosoft/ishell"
	"github.com/asdine/storm"
	"github.com/caarlos0/env"
	"github.com/pressly/chi"
	"github.com/pressly/chi/middleware"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type EnvConfig struct {
	JWT_ISSUER string `env:"RESIN_DEVICE_UUID" envDefault:"DEV"`
	RESIN      bool   `env:"RESIN" envDefault:"0"`
	SRCDIR     string `env:"SRCDIR" envDefault:"."`
	DB         *storm.DB
	Conductor  *Conductor
}

var (
	ENV *EnvConfig
)

func init() {
	// Load main config
	ENV = new(EnvConfig)
	env.Parse(ENV)

	// setup database
	// make sure to init all of the structs

	// get db path, this depends on if we are running on a resin device
	var dbFile string
	if ENV.RESIN {
		dbFile = "/data/live.db"
	} else {
		dbFile, _ = filepath.Abs("./tmp/dev.db")
		dir := filepath.Dir(dbFile)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			os.Mkdir(dir, 0755)
		}
	}

	db, err := openDb(dbFile)
	if err != nil {
		panic(err)
	}
	ENV.DB = db

	return
}

func main() {
	port := "0.0.0.0:80"

	//r := mux.NewRouter()
	//r.StrictSlash(true)
	//r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	//
	//r.PathPrefix("/ws/").Handler(_signaling.Handler())

	r := chi.NewRouter()

	// A good base middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.RedirectSlashes)
	r.Use(middleware.Recoverer) // make sure this is last

	defer ENV.DB.Close() // close database when finished

	// Setup the device properly so everything works as expected later
	// TODO: This should be a hybrid approach using the DB
	var filename string
	var err error
	if os.Getenv("RESIN") == "1" {
		println("Running on resin")
		filename = "/data/bbb_config.yaml"
	} else {
		filename, err = filepath.Abs(ENV.SRCDIR + "/bbb_config.yaml")
		if err != nil {
			panic(err)
		}
	}
	yamlFile, err := ioutil.ReadFile(filename)

	if err != nil {
		panic(fmt.Sprintf("Unable to read yaml file: %v", err))
	}

	var config DynastatConfig
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		panic(fmt.Sprintf("Unable to unmarshal yaml: %v", err))
	}

	var dynastat *Dynastat
	dynastat, err = NewDynastat(&config)
	if err != nil {
		panic(fmt.Sprintf("Unable to initialize dynastat: %v", err))
	}

	ENV.Conductor = new(Conductor)
	ENV.Conductor.Device = dynastat

	go ENV.Conductor.UpdateClients()

	//---
	// Create a local shell
	//---
	{
		shell := ishell.New()
		shell.Println("Dynastat development shell")
		shell.ShowPrompt(true)
		shell.AddCmd(&ishell.Cmd{
			Name: "createsuperuser",
			Help: "createsuperuser <email> <password>",
			Func: func(c *ishell.Context) {
				// disable the '>>>' for cleaner same line input.
				c.ShowPrompt(false)
				defer c.ShowPrompt(true) // yes, revert when done.

				// get email
				var email string
				if len(c.Args) >= 1 {
					email = c.Args[0]
				} else {
					c.Print("Email: ")
					email = c.ReadLine()
				}

				// get password
				var password string
				if len(c.Args) >= 2 {
					password = c.Args[1]
				} else {
					c.Print("Password: ")
					password = c.ReadPassword()
				}

				// create user
				user := &User{
					Email: email,
					Name:  email,
					Admin: true,
				}
				user.SetPassword([]byte(password))
				err := ENV.DB.Save(user)
				if err != nil {
					panic(err)
				}

				c.Println("Superuser created")
			},
		})
		go shell.Start()
	}

	//---
	// Build the API routes
	//---
	r.Route("/api", func(r chi.Router) {
		// login
		r.Post("/login", Login)

		r.Route("/", func(r chi.Router) {
			// Seek, verify and validate JWT tokens
			r.Use(ValidateJWT)

			r.Get("/foo", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("Success"))
			})

			r.Get("/refresh_token", JWTRefresh)
		})

	})

	// Add websocket routes
	r.Route("/ws", func(r chi.Router) {
		if ENV.RESIN {
			// Enable JWT validation in production
			r.Use(ValidateJWT)
		}

		r.Get("/echo", EchoHandler)
		r.Get("/signal", WebRTCSignalHandler)
	})

	// add static base routes
	r.FileServer("/", http.Dir(ENV.SRCDIR+"/html"))
	r.FileServer("/static", http.Dir(ENV.SRCDIR+"/static"))

	fmt.Println("Listening on port", port)
	if err := http.ListenAndServe(port, r); err != nil {
		log.Fatal(err)
	}
}

func openDb(dbFile string) (db *storm.DB, err error) {
	db, err = storm.Open(dbFile)
	if err != nil {
		return
	}

	// call inits for each type
	if err := db.Init(&User{}); err != nil {
		return nil, err
	}

	return
}
