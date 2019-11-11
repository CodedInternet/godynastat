package main

import (
	"flag"
	"fmt"
	"github.com/CodedInternet/godynastat/comms"
	. "github.com/CodedInternet/godynastat/onboard"
	"github.com/abiosoft/ishell"
	"github.com/asdine/storm/v3"
	"github.com/caarlos0/env/v6"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type EnvConfig struct {
	JWT_ISSUER string `env:"RESIN_DEVICE_UUID" envDefault:"DEV"`
	RESIN      bool   `env:"RESIN" envDefault:"0"`
	DEBUG      bool   `env:"DEBUG" envDefault:"0"`
	SRCDIR     string `env:"SRCDIR" envDefault:"."`
	HTMLDIR    string `env:"HTMLDIR" envDefault:"./frontend/dist/"`
	DB         *storm.DB
	Conductor  *comms.Conductor
	Simulated  bool
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
	//r := mux.NewRouter()
	//r.StrictSlash(true)
	//r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static/"))))
	//
	//r.PathPrefix("/ws/").Handler(_signaling.Handler())

	// process flags
	simulated := flag.Bool("sim", false, "Run the device in simulator mode)")
	port := flag.String("port", "0.0.0.0:80", "Specify the ip:port to listen on")
	flag.Parse()

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
	if ENV.RESIN {
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

	var dynastat *ActuatorDynastat

	ENV.Simulated = *simulated
	//if ENV.Simulated {
	//	println("Creating simulator")
	//	dynastat = NewDynastatSimulator(&config)
	//} else {
	//	dynastat, err = NewDynastat(&config)
	//	if err != nil {
	//		panic(fmt.Sprintf("Unable to initialize dynastat: %v", err))
	//	}
	//}
	dynastat, err = NewActuatorDynastat(config)

	ENV.Conductor = new(comms.Conductor)
	ENV.Conductor.Device = dynastat

	go ENV.Conductor.UpdateClients()

	//---
	// Create a local shell
	//---
	{
		platformNames := func([]string) []string {
			keys := make([]string, len(dynastat.Platforms))
			for k := range dynastat.Platforms {
				keys = append(keys, k)
			}
			return keys
		}

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

		// Add device specific commands
		shell.AddCmd(&ishell.Cmd{
			Name:      "move",
			Completer: platformNames,
			Help:      "move <name> <height> <tilt> <roll>",
			Func: func(c *ishell.Context) {
				name := c.Args[0]
				height, _ := strconv.Atoi(c.Args[1])
				tilt, _ := strconv.Atoi(c.Args[2])
				roll, _ := strconv.Atoi(c.Args[3])
				c.Printf("Moving platform %s to H:%d T:%d R:%d\n", name, height, tilt, roll)
				err = dynastat.SetHeight(name, float64(height))
				if err != nil {
					c.Err(err)
				}
				err = dynastat.SetRotation(name, float64(roll), float64(tilt))
				if err != nil {
					c.Err(err)
				}
			},
		})

		shell.AddCmd(&ishell.Cmd{
			Name:      "height",
			Completer: platformNames,
			Help:      "height <name> <height> <tilt> <roll>",
			Func: func(c *ishell.Context) {
				name := c.Args[0]
				height, _ := strconv.Atoi(c.Args[1])
				c.Printf("Moving platform %s to H:%d\n", name, height)
				err = dynastat.SetHeight(name, float64(height))
				if err != nil {
					c.Err(err)
					println(err)
				}
			},
		})

		shell.AddCmd(&ishell.Cmd{
			Name:      "home",
			Completer: platformNames,
			Help:      "home <Motor>",
			Func: func(c *ishell.Context) {
				name := string(c.Args[0])
				c.Printf("Homing Motor %s\n", name)
				dynastat.SetHeight(name, 0)
				dynastat.SetRotation(name, 0, 0)
			},
		})
		//shell.AddCmd(&ishell.Cmd{
		//	Name: "state",
		//	Help: "Reads the current state of the device",
		//	Func: func(c *ishell.Context) {
		//		c.Println("Getting state")
		//		state, err := dynastat.GetState()
		//		c.Printf("#v #v", state, err)
		//	},
		//})

		//shell.AddCmd(&ishell.Cmd{
		//	Name: "control",
		//	Func: func(c *ishell.Context) {
		//		buf := make([]byte, 2)
		//		dynastat.SensorBus.Get(0x20, 3, buf)
		//		val := binary.LittleEndian.Uint16(buf)
		//		c.Printf("0x%X\n", val)
		//
		//		for i := 0; i <= 10; i++ {
		//			c.Printf("Match: %d & %d = %v\n", val, i, val&(1<<uint16(i)) == 0)
		//		}
		//	},
		//})

		//{
		//	// Calibration specific commands
		//	calCmd := &ishell.Cmd{
		//		Name: "cal",
		//		Help: "calibrate a motor",
		//	}
		//
		//	calCmd.AddCmd(&ishell.Cmd{
		//		Name:      "move",
		//		Help:      "Move a motor to a specified absolute value",
		//		Completer: platformNames,
		//		Func: func(c *ishell.Context) {
		//			name := c.Args[0]
		//			position, _ := strconv.Atoi(c.Args[1])
		//
		//			dynastat.GotoMotorRaw(name, position)
		//		},
		//	})
		//
		//	calCmd.AddCmd(&ishell.Cmd{
		//		Name:      "write",
		//		Help:      "Write the current absolute value for a motor",
		//		Completer: platformNames,
		//		Func: func(c *ishell.Context) {
		//			name := c.Args[0]
		//			position, _ := strconv.Atoi(c.Args[1])
		//
		//			dynastat.WriteMotorRaw(name, position)
		//		},
		//	})
		//
		//	calCmd.AddCmd(&ishell.Cmd{
		//		Name:      "low",
		//		Help:      "Set the current position as the low value for a motor",
		//		Completer: platformNames,
		//		Func: func(c *ishell.Context) {
		//			name := c.Args[0]
		//			dynastat.RecordMotorLow(name)
		//		},
		//	})
		//	calCmd.AddCmd(&ishell.Cmd{
		//		Name:      "high",
		//		Help:      "Set the current position as the high value for a motor",
		//		Completer: platformNames,
		//		Func: func(c *ishell.Context) {
		//			name := c.Args[0]
		//			dynastat.RecordMotorHigh(name)
		//		},
		//	})
		//
		//	calCmd.AddCmd(&ishell.Cmd{
		//		Name:      "home",
		//		Help:      "Locate the home position and record the value in the config",
		//		Completer: platformNames,
		//		Func: func(c *ishell.Context) {
		//			if len(c.Args) != 2 {
		//				c.Err(errors.New("Incorrect number of arguments. Usage: cal home <motor_name> <reverse>"))
		//				return
		//			}
		//			name := c.Args[0]
		//			reverse, _ := strconv.ParseBool(c.Args[1])
		//
		//			c.ProgressBar().Indeterminate(true)
		//			c.ProgressBar().Start()
		//			pos, err := dynastat.RecordMotorHome(name, reverse)
		//			c.ProgressBar().Stop()
		//
		//			if err != nil {
		//				c.Err(err)
		//			}
		//
		//			c.Printf("Motor %s home located at %d\n", name, pos)
		//		},
		//	})
		//
		//	calCmd.AddCmd(&ishell.Cmd{
		//		Name: "commit",
		//		Help: "Commit the current config to disk",
		//		Func: func(c *ishell.Context) {
		//			yml, _ := yaml.Marshal(dynastat.GetConfig())
		//			ioutil.WriteFile(filename, yml, 0744)
		//		},
		//	})
		//
		//	shell.AddCmd(calCmd)
		//}

		// Start an instance of the shell so it can be controlled from the CLI
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
		if ENV.RESIN && !ENV.DEBUG {
			// Enable JWT validation in production
			r.Use(ValidateJWT)
		} else {
			fmt.Println("Running in debug mode. Authentication disabled.")
		}

		r.Get("/echo", EchoHandler)
		r.Get("/signal", WebRTCSignalHandler)
	})

	// add static base routes
	FileServer(r, "/", http.Dir(ENV.HTMLDIR))

	fmt.Println("Listening on port", *port)
	if err := http.ListenAndServe(*port, r); err != nil {
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

// FileServer conveniently sets up a http.FileServer handler to serve
// static files from a http.FileSystem.
func FileServer(r chi.Router, path string, root http.FileSystem) {
	if strings.ContainsAny(path, "{}*") {
		panic("FileServer does not permit URL parameters.")
	}

	fs := http.StripPrefix(path, http.FileServer(root))

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	}))
}
