// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"path/filepath"
	"time"
	"net/http"
	"database/sql"
	"flag"
	"go-mssqldb"
	"regexp"
  	"net/url"
  	"strconv"

	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
	"golang.org/x/sys/windows/svc"
)

var elog debug.Log

type myservice struct{}

var (
  serviceCmd	= flag.String("serviceCmd", "acaVaelComando", "Control del servicio")
  debugCmd      = flag.Bool("debugCmd", false, "Habilitar depuracion")
  password      = flag.String("password", "acaVaLaClave", "DB password")
  port     *int = flag.Int("port", 1433, "DB port")
  server        = flag.String("server", "acaVaElServidor", "DB server")
  user          = flag.String("user", "acaVaElUsuario", "DB user")
  database      = flag.String("database", "acaVaLaBase", "DB name")
  bolilla  *int = flag.Int("bollila", 1, "Bolilla inicial")
  conn *sql.DB
  err error
  servidorPPress = []string{"NULO PARA CORREGIR DEFASAJE INDICES","http://cpl-printw01:8080/extraer_pdf","http://cpl-printw01:8080/extraer_pdf","http://cpl-printw02:8080/extraer_pdf","http://cpl-printw02:8080/extraer_pdf"}
  puertoHttp    = ":8082"
)

var pathBalance = regexp.MustCompile("^/(balance/balance.php)$")
var pathConsulta = regexp.MustCompile("^/(consulta/consulta.php)$")

func balanceador(w http.ResponseWriter, r *http.Request) {
  //fmt.Fprintf(w, "<html><head><title>Servicio de consulta masiva de comprobantes</title></head></html>")
  if *debugCmd {
    fmt.Printf("********************************************************************************\nHora inicio balance: %s\n",time.Now())
  }

  consultaBase := url.Values{}
  resultadoBase := url.Values{}
  // verificacion de path url, si no es corecto, retorna http 403
  m := pathBalance.FindStringSubmatch(r.URL.Path)
  if m == nil {
    http.NotFound(w, r)
    fmt.Printf(" BAD REQUEST - Path=%s - bolilla=%d", r.URL.Path, *bolilla)
    r.ParseForm()
    for key, values := range r.Form {
      fmt.Printf(" - %s: %s", key,values)
    }
    print("\n")
    return
  }
  // variables utilizadas para consulta en base
  consultaBase.Set("numeroComprobante",r.FormValue("d"))
  consultaBase.Set("prefijoComprobante",r.FormValue("p"))
  consultaBase.Set("cuentaComprobante",r.FormValue("c"))
  if r.FormValue("t") == "cen"{
  	consultaBase.Set("distribuidoraComprobante","C")
  }else{
  	consultaBase.Set("distribuidoraComprobante","Y")
  }
  consultaBase.Set("observacionesComprobante",r.FormValue("o"))
  // se arma la variable que contiene la consulta
  query := "Exec ExtraePDF '" + consultaBase.Get("prefijoComprobante") + "','" + consultaBase.Get("numeroComprobante") + "','" + consultaBase.Get("cuentaComprobante") + "','" + consultaBase.Get("distribuidoraComprobante") + "'"
  if *debugCmd {
    fmt.Printf("Consulta: %s\n",query)
  }
  // se prepara la query
  stmt, err := conn.Prepare(query)
  if err != nil {
    // log.Fatal("Prepare failed:", err.Error())
    http.NotFound(w, r)
    fmt.Printf(" Error al preparar query SQL - Path=%s - bolilla=%d\n Detalle:%s", r.URL.Path, *bolilla, err.Error())
    r.ParseForm()
    for key, values := range r.Form {
      fmt.Printf(" - %s: %s", key,values)
    }
    print("\n")
    return
  }
  defer stmt.Close()
  // se almacena en forma de renglon la respuesta de la base
  row := stmt.QueryRow()
  // variables necesarias para almacenar los datos de la columnas de la consulta
  var servidor string
  var ruta string
  var distribuidora string
  var periodo string
  var nombre string
  var prefijo_comp string
  var numero_comp string
  var tipo_comp string
  var formulario string
  var numero_cta string
  var pagina int
  var time_factura string
  // scan realiza un parseo de la respuesta y asigna los valores a las variables
  err = row.Scan(&servidor, &ruta, &distribuidora, &periodo, &nombre, &prefijo_comp, &numero_comp, &tipo_comp, &formulario, &numero_cta, &pagina, &time_factura)
  if err != nil {
    // log.Fatal("Scan failed:", err.Error())
    http.NotFound(w, r)
    fmt.Printf(" Error al parsear resultado query SQL - Path=%s - bolilla=%d\n Detalle:%s", r.URL.Path, *bolilla, err.Error())
    r.ParseForm()
    for key, values := range r.Form {
      fmt.Printf(" - %s: %s", key,values)
    }
    print("\n")
    return
  }
  // Como se necesita un typo url.Values para el form del post a PPress, se arma con cada una de las variables
  resultadoBase.Set("servidor",servidor)
  resultadoBase.Set("ruta",ruta)
  resultadoBase.Set("periodo",periodo)
  resultadoBase.Set("nombre",nombre)
  resultadoBase.Set("pagina",strconv.Itoa(pagina))
  resultadoBase.Set("formulario",formulario)
  resultadoBase.Set("observaciones",r.FormValue("o"))

  if *debugCmd {
    // consultaBase["numeroComprobante"] <--> r.FormValue("d")
    // consultaBase["prefijoComprobante"] <--> r.FormValue("p")
    // consultaBase["cuentaComprobante"] <--> r.FormValue("c")
    // consultaBase["distribuidoraComprobante"] <--> r.FormValue("t") / cen (c)- cuy (y)
    // consultaBase["observacionesComprobante"] <--> r.FormValue("o")
    fmt.Printf("Datos HTTP: cuenta=%s, prefijo=%s, numero=%s, distribuidora=%s, observaciones=%s, bolilla=%d\n", consultaBase.Get("cuentaComprobante"), consultaBase.Get("prefijoComprobante"), consultaBase.Get("numeroComprobante"), consultaBase.Get("distribuidoraComprobante"), consultaBase.Get("observacionesComprobante"), *bolilla)
    // resultadoBase["servidor"] <--> servidor
    // resultadoBase["ruta"] <--> ruta
    // resultadoBase["periodo"] <--> periodo
    // resultadoBase["nombre"] <--> nombre
    // resultadoBase["pagina"] <--> pagina
    fmt.Printf("Datos Base: archivo=%s%s%s%s, pagina=%s\n", resultadoBase.Get("servidor"), resultadoBase.Get("ruta"), resultadoBase.Get("periodo"), resultadoBase.Get("nombre"), resultadoBase.Get("pagina"))
  }
  // Post con formulario a PPress. Se le pasa 
  resp, err := http.PostForm(servidorPPress[*bolilla], resultadoBase)
  // El codigo 200 es Ok
  if resp.StatusCode != 200{
  	// log.Fatal("Bad response:", err.Error())
    http.NotFound(w, r)
    fmt.Printf(" Error de comunicacion con PPress - Path=%s - bolilla=%d\n Detalle:%s", r.URL.Path, *bolilla, err.Error())
    r.ParseForm()
    for key, values := range r.Form {
      fmt.Printf(" - %s: %s", key,values)
    }
    print("\n")
    return
  }

  // FORMATO DE NOMBRE ARCHIVO
  // fact-ecogas-pppp-DDDDDDDD-CCCCCCCC-xxx.pdf
  // pppp es punto de venta (prefijo)
  // DDDDDDDD es numero de factura (numero)
  // CCCCCCCC es numero de cuenta
  // xxxx es cen o cuy sin comillas

  nombreFinal := fmt.Sprintf("fact-ecogas-%s-%s-%s-%s",consultaBase.Get("prefijoComprobante"),consultaBase.Get("numeroComprobante"),consultaBase.Get("cuentaComprobante"),consultaBase.Get("distribuidoraComprobante"))
  // hay dos metodos inline y attachment, el primero se muestra en pantalla, el segundo se descarga
  // se escriben estos dos parametros para que el browser entienda que es un pdf y se asigna el nombre de archivo
  //w.Header().Set("Content-Disposition", "attachment; filename="+resultadoBase.Get("nombre"))
  w.Header().Set("Content-Disposition", "inline; filename="+nombreFinal)
  w.Header().Set("Content-Type", "application/pdf")
  // la respuesta se copia directamente lo que responde del postform ppress 
  resp.Write(w)
  defer resp.Body.Close()
  // incremento del valor de bolilla para distribucion de carga
  *bolilla += 1
  if (*bolilla >= 5)||(*bolilla < 1){
    *bolilla = 1
  }
  if *debugCmd {
    fmt.Printf("Hora finalizacion balance: %s\n",time.Now())
  }
}

func consultador(w http.ResponseWriter, r *http.Request) {
  //fmt.Fprintf(w, "<html><head><title>Servicio de consulta masiva de comprobantes</title></head></html>")
  if *debugCmd {
    fmt.Printf("********************************************************************************\nHora inicio consulta: %s\n",time.Now())
  }

  consultaBase := url.Values{}
  // verificacion de path url, si no es corecto, retorna http 403
  m := pathConsulta.FindStringSubmatch(r.URL.Path)
  if m == nil {
    http.NotFound(w, r)
    fmt.Printf(" BAD REQUEST - Path=%s - bolilla=%d\n Detalle:%s", r.URL.Path, *bolilla, err.Error())
    r.ParseForm()
    for key, values := range r.Form {
      fmt.Printf(" - %s: %s", key,values)
    }
    print("\n")
    return
  }
  // variables utilizadas para consulta en base
  consultaBase.Set("numeroComprobante",r.FormValue("d"))
  consultaBase.Set("prefijoComprobante",r.FormValue("p"))
  consultaBase.Set("cuentaComprobante",r.FormValue("c"))
  if r.FormValue("t") == "cen"{
  	consultaBase.Set("distribuidoraComprobante","C")
  }else{
  	consultaBase.Set("distribuidoraComprobante","Y")
  }
  consultaBase.Set("existeComprobante",r.FormValue("e"))
  // se arma la variable que contiene la consulta
  query := "Exec ConsultaExiste '"+ consultaBase.Get("prefijoComprobante") +"','"+ consultaBase.Get("numeroComprobante") +"','"+ consultaBase.Get("cuentaComprobante") +"','" + consultaBase.Get("distribuidoraComprobante") + "'"
  if *debugCmd {
    fmt.Printf("Consulta: %s\n",query)
  }
  // se prepara la query
  stmt, err := conn.Prepare(query)
  if err != nil {
    //log.Fatal("Prepare failed:", err.Error())
    http.NotFound(w, r)
    fmt.Printf(" Error al preparar query SQL - Path=%s - bolilla=%d\n Detalle:%s", r.URL.Path, *bolilla, err.Error())
    r.ParseForm()
    for key, values := range r.Form {
      fmt.Printf(" - %s: %s", key,values)
    }
    print("\n")
    return
  }
  defer stmt.Close()
  // se almacena en forma de renglon la respuesta de la base
  row := stmt.QueryRow()
  // variables necesarias para almacenar los datos de la columnas de la consulta
  var respuesta string
  // scan realiza un parseo de la respuesta y asigna los valores a las variables
  err = row.Scan(&respuesta)
  if err != nil {
    //log.Fatal("Scan failed:", err.Error())
    http.NotFound(w, r)
    fmt.Printf(" Error al parsear resultado query SQL - Path=%s - bolilla=%d\n Detalle:%s", r.URL.Path, *bolilla, err.Error())
    r.ParseForm()
    for key, values := range r.Form {
      fmt.Printf(" - %s: %s", key,values)
    }
    print("\n")
    return
  }

  if *debugCmd {
    // consultaBase["numeroComprobante"] <--> r.FormValue("d")
    // consultaBase["prefijoComprobante"] <--> r.FormValue("p")
    // consultaBase["cuentaComprobante"] <--> r.FormValue("c")
    // consultaBase["distribuidoraComprobante"] <--> r.FormValue("t") / cen (c)- cuy (y)
    // consultaBase["observacionesComprobante"] <--> r.FormValue("o")
    fmt.Printf("Datos HTTP: cuenta=%s, prefijo=%s, numero=%s, distribuidora=%s, existe=%s, bolilla=%d\n", consultaBase.Get("cuentaComprobante"), consultaBase.Get("prefijoComprobante"), consultaBase.Get("numeroComprobante"), consultaBase.Get("distribuidoraComprobante"), consultaBase.Get("existeComprobante"), *bolilla)
    // resultadoBase["servidor"] <--> servidor
    fmt.Printf("Datos Base: respuesta=%s\n", respuesta)
  }

  // respuesta si existe o no el comprobante directamente de la respuesta SQL
  w.Write([]byte(respuesta))
  if *debugCmd {
    fmt.Printf("Hora finalizacion consulta: %s\n",time.Now())
  }
}

func (m *myservice) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	changes <- svc.Status{State: svc.StartPending}
	fasttick := time.Tick(500 * time.Millisecond)
	slowtick := time.Tick(2 * time.Second)
	tick := fasttick
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	elog.Info(1, strings.Join(args, "-"))

	go func() {
 
  connString := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%d;database=%s", *server, *user, *password, *port, *database)
  if *debugCmd {
    fmt.Printf(" connString:%s\n", connString)
  }

  conn, err = sql.Open("mssql", connString)
  if err != nil {
    log.Fatal("No se pudo abrir la conexion a la BD:", err.Error())
  }
  defer conn.Close() // cierra la conexion al final de la ejecucion del main

  // workaround para indicar que se usa mssql sino falla la compilacion
  dec, err2 := mssql.Float64ToDecimal(1.00)
  if err2 != nil {
    log.Fatal("No se pudo convertir a float:", err.Error())
  }
  print(dec.String()+"\n")
  // fin workaround
  
  http.HandleFunc("/balance/", balanceador)
  http.HandleFunc("/consulta/", consultador)
  if err := http.ListenAndServe(puertoHttp, nil); err != nil {
    panic(err)
  }


	}()

loop:
	for {
		select {
		 case <-tick:
		 	// nada
		 case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				break loop
			case svc.Pause:
				changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
				tick = slowtick
			case svc.Continue:
				changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
				tick = fasttick
			default:
				elog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

func runService(name string, isDebug bool) {
	var err error
	if isDebug {
		elog = debug.New(name)
	} else {
		elog, err = eventlog.Open(name)
		if err != nil {
			return
		}
	}
	defer elog.Close()

	elog.Info(1, fmt.Sprintf("starting %s service", name))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &myservice{})
	if err != nil {
		elog.Error(1, fmt.Sprintf("%s service failed: %v", name, err))
		return
	}
	elog.Info(1, fmt.Sprintf("%s service stopped", name))
}

func usage(errmsg string) {
	fmt.Fprintf(os.Stderr,
		"%s\n\n"+
			"usage: %s <command>\n"+
			"       where <command> is one of\n"+
			"       install, remove, debug, start, stop, pause or continue.\n",
		errmsg, os.Args[0])
	os.Exit(2)
}

func exePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("%s is directory", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			err = fmt.Errorf("%s is directory", p)
		}
	}
	return "", err
}


func installService(name, dispName string) error {
	exepath, err := exePath()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", name)
	}
	s, err = m.CreateService(name, exepath, mgr.Config{DisplayName: dispName,Description: "Servicio HTTP encargado de recibir las consultas del sitio web, balancear la carga y extraer comprobantes en formato PDF en conjunto con un servidor SQL y PlanetPress Suite.\nDesarrollado en Doxer S.A. por Ingeniero Yamil Taler: https://ytaler.github.io"}, "is", "auto-started")
	if err != nil {
		return err
	}
	defer s.Close()
	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return fmt.Errorf("SetupEventLogSource() failed: %s", err)
	}
	return nil
}

func removeService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("service %s is not installed", name)
	}
	defer s.Close()
	err = s.Delete()
	if err != nil {
		return err
	}
	err = eventlog.Remove(name)
	if err != nil {
		return fmt.Errorf("RemoveEventLogSource() failed: %s", err)
	}
	return nil
}

func startService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	err = s.Start("is", "manual-started")
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}

func controlService(name string, c svc.Cmd, to svc.State) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}
	timeout := time.Now().Add(10 * time.Second)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service status: %v", err)
		}
	}
	return nil
}

func handler(w http.ResponseWriter, r *http.Request) {
	var name string

	found := r.URL.Query().Get("name")
	if found != "" {
		name = found
	} else {
		name = "world"
	}

	fmt.Fprintf(w, "Hello, %s!", name)
}

func main() {
	const svcName = "GoWebServer"

  	flag.Parse()

if *debugCmd {
    fmt.Printf("********************************************************************************\n password:%s\n", *password)
    fmt.Printf(" port:%d\n", *port)
    fmt.Printf(" server:%s\n", *server)
    fmt.Printf(" user:%s\n", *user)
    fmt.Printf(" database:%s\n", *database)
    fmt.Printf(" bolilla:%d\n", *bolilla)
  }

	isIntSess, err := svc.IsAnInteractiveSession()
	if err != nil {
		log.Fatalf("failed to determine if we are running in an interactive session: %v", err)
	}
	if !isIntSess {
		print("Is not an interactive session")
		runService(svcName, false)
		return
	}

	if len(os.Args) < 2 {
		usage("no command specified")
	}

	cmd := strings.ToLower(*serviceCmd)
	switch cmd {
	case "debug":
		runService(svcName, true)
		return
	case "install":
		err = installService(svcName, "Go Web Server")
	case "remove":
		err = removeService(svcName)
	case "start":
		err = startService(svcName)
	case "stop":
		err = controlService(svcName, svc.Stop, svc.Stopped)
	case "pause":
		err = controlService(svcName, svc.Pause, svc.Paused)
	case "continue":
		err = controlService(svcName, svc.Continue, svc.Running)
	default:
		usage(fmt.Sprintf("invalid command %s", cmd))
	}
	if err != nil {
		log.Fatalf("failed to %s %s: %v", cmd, svcName, err)
	}
	return
}