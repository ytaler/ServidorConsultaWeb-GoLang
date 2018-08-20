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
  "go-mssqldb"
  "regexp"
  "net/url"
  "./out"
  
  "golang.org/x/sys/windows/svc/debug"
  "golang.org/x/sys/windows/svc/eventlog"
  "golang.org/x/sys/windows/svc/mgr"
  "golang.org/x/sys/windows/svc"
)

const svcName = "GoWebServer"

type myservice struct{}

type jsonObject struct {
    Config serviceConfig
}

type serviceConfig struct {
  Password  string
  Port  int
  Server  string
  User  string
  Database  string
  Bolilla  int
  HttpPort  int
}

var (  
  jsontype jsonObject
  elog debug.Log
  conn *sql.DB
  err error
  servidorPPress = []string{"NULO PARA CORREGIR DEFASAJE INDICES","http://cpl-printw01:8080/extraer_pdf","http://cpl-printw02:8080/extraer_pdf","http://cpl-printw01:8080/extraer_pdf","http://cpl-printw02:8080/extraer_pdf"}
  bolilla int = 1
  wsDebug bool = false
  pathBalance = regexp.MustCompile("^/(balance/balance.php)$")
  pathConsulta = regexp.MustCompile("^/(consulta/consulta.php)$")
)


func balanceador(w http.ResponseWriter, r *http.Request) {
  mensajeError := ""
  //fmt.Fprintf(w, "<html><head><title>Servicio de consulta masiva de comprobantes</title></head></html>")
  if wsDebug {
    print("********************************************************************************\n")
    fmt.Printf("%s.info(2): Hora inicio balance: %s\n", svcName,time.Now())
    out.LogString(fmt.Sprintf("********************************************************************************"))
    out.LogString(fmt.Sprintf("%s.info(2): Hora inicio balance: %s", svcName,time.Now()))
  }
  consultaBase := url.Values{}
  resultadoBase := url.Values{}
  // verificacion de path url, si no es corecto, retorna http 403
  m := pathBalance.FindStringSubmatch(r.URL.Path)
  if m == nil{
    mensajeError = fmt.Sprintf(" BAD REQUEST - Path=%s - bolilla=%d", r.URL.Path, bolilla)
    r.ParseForm()
    for key, values := range r.Form {
        mensajeError = mensajeError + fmt.Sprintf(" - %s: %s", key,values)
    }
    if wsDebug {
      fmt.Printf("%s.info(2): %s\n", svcName, mensajeError)
    }
	out.LogString(fmt.Sprintf("%s.info(2): %s", svcName, mensajeError))
    w.Write([]byte("<html><head><title>Servicio de consulta masiva de comprobantes</title></head><body>"+mensajeError+"</body></html>"))  
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
  if wsDebug {
    fmt.Printf("%s.info(2): Consulta: %s\n", svcName, query)
	out.LogString(fmt.Sprintf("%s.info(2): Consulta: %s", svcName, query))
  }
  // se prepara la query
  stmt, err := conn.Prepare(query)
  if err != nil{
    mensajeError = fmt.Sprintf(" Error al preparar query SQL - Path=%s - bolilla=%d\n Detalle:%s", r.URL.Path, bolilla, err.Error())
    r.ParseForm()
    for key, values := range r.Form {
        mensajeError = mensajeError + fmt.Sprintf(" - %s: %s", key,values)
    }
    if wsDebug{
      fmt.Printf("%s.info(2): %s\n", svcName, mensajeError)
    }
	out.LogString(fmt.Sprintf("%s.info(2): %s", svcName, mensajeError))
    w.Write([]byte("<html><head><title>Servicio de consulta masiva de comprobantes</title></head><body>"+mensajeError+"</body></html>"))  
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
    mensajeError = fmt.Sprintf(" Error al parsear resultado query SQL - Path=%s - bolilla=%d\n Detalle:%s", r.URL.Path, bolilla, err.Error())
    r.ParseForm()
    for key, values := range r.Form {
        mensajeError = mensajeError + fmt.Sprintf(" - %s: %s", key,values)
    }
    if wsDebug {
      fmt.Printf("%s.info(2): %s\n", svcName, mensajeError)
    }
	out.LogString(fmt.Sprintf("%s.info(2): %s", svcName, mensajeError))
    w.Write([]byte("<html><head><title>Servicio de consulta masiva de comprobantes</title></head><body>"+mensajeError+"</body></html>"))  
    return
  }

  // Como se necesita un typo url.Values para el form del post a PPress, se arma con cada una de las variables
  resultadoBase.Set("servidor",servidor)
  resultadoBase.Set("ruta",ruta)
  resultadoBase.Set("periodo",periodo)
  resultadoBase.Set("nombre",nombre)
  resultadoBase.Set("pagina",fmt.Sprintf("%d",pagina))
  resultadoBase.Set("formulario",formulario)
  resultadoBase.Set("observaciones",r.FormValue("o"))

  if wsDebug{
    // consultaBase["numeroComprobante"] <--> r.FormValue("d")
    // consultaBase["prefijoComprobante"] <--> r.FormValue("p")
    // consultaBase["cuentaComprobante"] <--> r.FormValue("c")
    // consultaBase["distribuidoraComprobante"] <--> r.FormValue("t") / cen (c)- cuy (y)
    // consultaBase["observacionesComprobante"] <--> r.FormValue("o")
    fmt.Printf("%s.info(2): Datos HTTP: cuenta=%s, prefijo=%s, numero=%s, distribuidora=%s, observaciones=%s, bolilla=%d\n", svcName, consultaBase.Get("cuentaComprobante"), consultaBase.Get("prefijoComprobante"), consultaBase.Get("numeroComprobante"), consultaBase.Get("distribuidoraComprobante"), consultaBase.Get("observacionesComprobante"), bolilla)
    out.LogString(fmt.Sprintf("%s.info(2): Datos HTTP: cuenta=%s, prefijo=%s, numero=%s, distribuidora=%s, observaciones=%s, bolilla=%d", svcName, consultaBase.Get("cuentaComprobante"), consultaBase.Get("prefijoComprobante"), consultaBase.Get("numeroComprobante"), consultaBase.Get("distribuidoraComprobante"), consultaBase.Get("observacionesComprobante"), bolilla))	
    // resultadoBase["servidor"] <--> servidor
    // resultadoBase["ruta"] <--> ruta
    // resultadoBase["periodo"] <--> periodo
    // resultadoBase["nombre"] <--> nombre
    // resultadoBase["pagina"] <--> pagina
    fmt.Printf("%s.info(2): Datos Base: archivo=%s%s%s%s, pagina=%s\n", svcName, resultadoBase.Get("servidor"), resultadoBase.Get("ruta"), resultadoBase.Get("periodo"), resultadoBase.Get("nombre"), resultadoBase.Get("pagina"))
    out.LogString(fmt.Sprintf("%s.info(2): Datos Base: archivo=%s%s%s%s, pagina=%s", svcName, resultadoBase.Get("servidor"), resultadoBase.Get("ruta"), resultadoBase.Get("periodo"), resultadoBase.Get("nombre"), resultadoBase.Get("pagina")))	
  }

  // Post con formulario a PPress. Se le pasa 
  resp, err := http.PostForm(servidorPPress[bolilla], resultadoBase)
  // El codigo 200 es Ok
  if (err != nil) || (resp.StatusCode != 200) {
    mensajeError = fmt.Sprintf(" Error de comunicacion con PPress - Path=%s - bolilla=%d\n Detalle:%s", r.URL.Path, bolilla, err.Error())
    r.ParseForm()
    for key, values := range r.Form {
        mensajeError = mensajeError + fmt.Sprintf(" - %s: %s", key,values)
    }
    if wsDebug {
      fmt.Printf("%s.info(2): %s\n", svcName, mensajeError)
    }
	out.LogString(fmt.Sprintf("%s.info(2): %s\n", svcName, mensajeError))
    w.Write([]byte("<html><head><title>Servicio de consulta masiva de comprobantes</title></head><body>"+mensajeError+"</body></html>"))  
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
  bolilla += 1
  if (bolilla >= 5)||(bolilla < 1){
    bolilla = 1
  }
  if wsDebug {
    fmt.Printf("%s.info(2): Hora finalizacion balance: %s\n", svcName, time.Now())
	out.LogString(fmt.Sprintf("%s.info(2): Hora finalizacion balance: %s", svcName, time.Now()))
  }
}

func consultador(w http.ResponseWriter, r *http.Request) {
  mensajeError := ""
  //fmt.Fprintf(w, "<html><head><title>Servicio de consulta masiva de comprobantes</title></head></html>")
  if wsDebug {
    print("********************************************************************************\n")
    fmt.Printf("%s.info(2): Hora inicio consulta: %s\n", svcName, time.Now())
    out.LogString(fmt.Sprintf("********************************************************************************"))
    out.LogString(fmt.Sprintf("%s.info(2): Hora inicio consulta: %s", svcName, time.Now()))
  }
  consultaBase := url.Values{}
  // verificacion de path url, si no es corecto, retorna http 403
  m := pathConsulta.FindStringSubmatch(r.URL.Path)
  if m == nil {
    mensajeError = fmt.Sprintf(" BAD REQUEST - Path=%s - bolilla=%d\n Detalle:%s", r.URL.Path, bolilla, err.Error())
    r.ParseForm()
    for key, values := range r.Form {
        mensajeError = mensajeError + fmt.Sprintf(" - %s: %s", key,values)
    }
    if wsDebug{
      fmt.Printf("%s.info(2): %s\n", svcName, mensajeError)
    }
	out.LogString(fmt.Sprintf("%s.info(2): %s", svcName, mensajeError))
    w.Write([]byte("<html><head><title>Servicio de consulta masiva de comprobantes</title></head><body>"+mensajeError+"</body></html>"))  
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
  if wsDebug{
    fmt.Printf("%s.info(2): Consulta: %s\n", svcName, query)
	out.LogString(fmt.Sprintf("%s.info(2): Consulta: %s", svcName, query))
  }
  // se prepara la query
  stmt, err := conn.Prepare(query)
  if err != nil{
    mensajeError = fmt.Sprintf(" Error al preparar query SQL - Path=%s - bolilla=%d\n Detalle:%s", r.URL.Path, bolilla, err.Error())
    r.ParseForm()
    for key, values := range r.Form {
        mensajeError = mensajeError + fmt.Sprintf(" - %s: %s", key,values)
    }
    if wsDebug{
      fmt.Printf("%s.info(2): %s\n", svcName, mensajeError)
    }
	out.LogString(fmt.Sprintf("%s.info(2): %s", svcName, mensajeError))
    w.Write([]byte("<html><head><title>Servicio de consulta masiva de comprobantes</title></head><body>"+mensajeError+"</body></html>"))  
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
    mensajeError = fmt.Sprintf(" Error al parsear resultado query SQL - Path=%s - bolilla=%d\n Detalle:%s", r.URL.Path, bolilla, err.Error())
    r.ParseForm()
    for key, values := range r.Form {
        mensajeError = mensajeError + fmt.Sprintf(" - %s: %s", key,values)
    }
    if wsDebug {
      fmt.Printf("%s.info(2): %s\n", svcName, mensajeError)
    }
	out.LogString(fmt.Sprintf("%s.info(2): %s", svcName, mensajeError))
    w.Write([]byte("<html><head><title>Servicio de consulta masiva de comprobantes</title></head><body>"+mensajeError+"</body></html>"))  
    return
  }  

  if wsDebug{
    // consultaBase["numeroComprobante"] <--> r.FormValue("d")
    // consultaBase["prefijoComprobante"] <--> r.FormValue("p")
    // consultaBase["cuentaComprobante"] <--> r.FormValue("c")
    // consultaBase["distribuidoraComprobante"] <--> r.FormValue("t") / cen (c)- cuy (y)
    // consultaBase["observacionesComprobante"] <--> r.FormValue("o")
    fmt.Printf("%s.info(2): Datos HTTP: cuenta=%s, prefijo=%s, numero=%s, distribuidora=%s, existe=%s, bolilla=%d\n", svcName, consultaBase.Get("cuentaComprobante"), consultaBase.Get("prefijoComprobante"), consultaBase.Get("numeroComprobante"), consultaBase.Get("distribuidoraComprobante"), consultaBase.Get("existeComprobante"), bolilla)
	out.LogString(fmt.Sprintf("%s.info(2): Datos HTTP: cuenta=%s, prefijo=%s, numero=%s, distribuidora=%s, existe=%s, bolilla=%d", svcName, consultaBase.Get("cuentaComprobante"), consultaBase.Get("prefijoComprobante"), consultaBase.Get("numeroComprobante"), consultaBase.Get("distribuidoraComprobante"), consultaBase.Get("existeComprobante"), bolilla))
    // resultadoBase["servidor"] <--> servidor
    fmt.Printf("%s.info(2): Datos Base: respuesta=%s\n", svcName, respuesta)
	out.LogString(fmt.Sprintf("%s.info(2): Datos Base: respuesta=%s", svcName, respuesta))
  }  
  // respuesta si existe o no el comprobante directamente de la respuesta SQL
  w.Write([]byte(respuesta))
  if wsDebug{
    fmt.Printf("%s.info(2): Hora finalizacion consulta: %s\n", svcName, time.Now())
	out.LogString(fmt.Sprintf("%s.info(2): Hora finalizacion consulta: %s", svcName, time.Now()))
  }
}

func (m *myservice) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
  out.LogString("Inicio Execute")
  const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
  changes <- svc.Status{State: svc.StartPending}
  fasttick := time.Tick(500 * time.Millisecond)
  slowtick := time.Tick(2 * time.Second)
  tick := fasttick
  changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
  elog.Info(1, strings.Join(args, "-"))
  
  jsontype.Config.Password =
  jsontype.Config.Port =
  jsontype.Config.Server =
  jsontype.Config.User =
  jsontype.Config.Database =
  jsontype.Config.Bolilla =
  jsontype.Config.HttpPort =

  if wsDebug{
    print("********************************************************************************\n")
    fmt.Printf("%s.info(2): password:%s\n",svcName, jsontype.Config.Password)
    fmt.Printf("%s.info(2): sql port:%d\n",svcName, jsontype.Config.Port)
    fmt.Printf("%s.info(2): http port:%d\n",svcName, jsontype.Config.HttpPort)
    fmt.Printf("%s.info(2): server:%s\n",svcName, jsontype.Config.Server)
    fmt.Printf("%s.info(2): user:%s\n",svcName, jsontype.Config.User)
    fmt.Printf("%s.info(2): database:%s\n",svcName, jsontype.Config.Database)
    fmt.Printf("%s.info(2): bolilla inicial:%d\n",svcName, bolilla)
  }
  
  out.LogString(fmt.Sprintf("%s.info(2): password:%s",svcName, jsontype.Config.Password))
  out.LogString(fmt.Sprintf("%s.info(2): sql port:%d",svcName, jsontype.Config.Port))
  out.LogString(fmt.Sprintf("%s.info(2): http port:%d",svcName, jsontype.Config.HttpPort))
  out.LogString(fmt.Sprintf("%s.info(2): server:%s",svcName, jsontype.Config.Server))
  out.LogString(fmt.Sprintf("%s.info(2): user:%s",svcName, jsontype.Config.User))
  out.LogString(fmt.Sprintf("%s.info(2): database:%s",svcName, jsontype.Config.Database))
  out.LogString(fmt.Sprintf("%s.info(2): bolilla inicial:%d",svcName, bolilla))

  go func(){
    out.LogString("Inicio Http Thread")
    connString := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%d;database=%s", jsontype.Config.Server, jsontype.Config.User, jsontype.Config.Password, jsontype.Config.Port, jsontype.Config.Database)

    if wsDebug{
      fmt.Printf("%s.info(2): connString:%s\n", svcName, connString)
    }
    out.LogString(fmt.Sprintf("%s.info(2): connString:%s", svcName, connString))
    conn, err = sql.Open("mssql", connString)
    if err != nil {
	  out.LogString(fmt.Sprintf("No se pudo abrir la conexion a la BD: %v", err))
      log.Fatal("No se pudo abrir la conexion a la BD:", err.Error())
    }
    defer conn.Close() // cierra la conexion al final de la ejecucion del main

    // workaround para indicar que se usa mssql sino falla la compilacion
    dec, err := mssql.Float64ToDecimal(1.00)
    if err != nil {
      log.Fatal("No se pudo convertir a float:", err.Error())
    }
    fmt.Printf("%s.info(0): Workaround value is %s\n", svcName, dec.String())
    // fin workaround
    if wsDebug{
      fmt.Printf("%s.info(0): starting balanceador handler\n", svcName)
    }
	out.LogString(fmt.Sprintf("%s.info(0): starting balanceador handler", svcName))
    http.HandleFunc("/balance/", balanceador)
    if wsDebug{
      fmt.Printf("%s.info(0): starting consultador handler\n", svcName)
    }
	out.LogString(fmt.Sprintf("%s.info(0): starting consultador handler", svcName))
    http.HandleFunc("/consulta/", consultador)
    if wsDebug{
      fmt.Printf("%s.info(0): starting http server\n", svcName)
    }
    out.LogString(fmt.Sprintf("%s.info(0): starting http server", svcName))
    if err := http.ListenAndServe(":"+fmt.Sprintf("%d",jsontype.Config.HttpPort), nil); err != nil {
	  out.LogString(fmt.Sprintf("Error al iniciar http listener: %v", err))
      panic(err)
    }
	out.LogString("Fin Http Thread")
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
  out.LogString("Fin Execute")
  return
}

func runService(name string, isDebug bool) {  
  out.LogString(fmt.Sprintf("Inicio runService, servicio: %s, debug: %s",name, isDebug))
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
    out.LogString(fmt.Sprintf("%s service failed: %v", name, err))
    elog.Error(1, fmt.Sprintf("%s service failed: %v", name, err))
    return
  }
  elog.Info(1, fmt.Sprintf("%s service stopped", name))
  out.LogString("Fin runService")
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
  out.LogString(fmt.Sprintf("Inicio installService, servicio: %s, dname: %s",name, dispName))
  exepath, err := exePath()
  if err != nil {
    out.LogString(fmt.Sprintf("Failed to get exepath: %v",err))
    return err
  }
  m, err := mgr.Connect()
  if err != nil {
    out.LogString(fmt.Sprintf("could not connect to service manager: %v", err))
    return err
  }
  defer m.Disconnect()
  s, err := m.OpenService(name)
  if err == nil {
    s.Close()
    out.LogString(fmt.Sprintf("service %s already exists", name))
    return fmt.Errorf("service %s already exists", name)
  }
  s, err = m.CreateService(name, exepath, mgr.Config{DisplayName: dispName,Description: "Servicio HTTP encargado de recibir las consultas del sitio web, balancear la carga y extraer comprobantes en formato PDF en conjunto con un servidor SQL y PlanetPress Suite.\nDesarrollado en Doxer S.A. por Ingeniero Yamil Taler: https://ytaler.github.io", StartType: mgr.StartAutomatic})
  if err != nil {
    out.LogString(fmt.Sprintf("Create service error: %v", err))
    return err
  }
  defer s.Close()
  err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
  if err != nil {
    s.Delete()
	out.LogString(fmt.Sprintf("SetupEventLogSource() failed: %s", err))	
    return fmt.Errorf("SetupEventLogSource() failed: %s", err)
  }
  out.LogString("Fin installService")
  return nil
}

func removeService(name string) error {
  out.LogString(fmt.Sprintf("Inicio removeService, servicio: %s",name))
  m, err := mgr.Connect()
  if err != nil {
    out.LogString(fmt.Sprintf("could not connect to service manager: %v", err))
    return err
  }
  defer m.Disconnect()
  s, err := m.OpenService(name)
  if err != nil {
    out.LogString(fmt.Sprintf("service %s is not installed", name))
    return fmt.Errorf("service %s is not installed", name)
  }
  defer s.Close()
  err = s.Delete()
  if err != nil {
    out.LogString(fmt.Sprintf("Delete failed: %v",err))
    return err
  }
  err = eventlog.Remove(name)
  if err != nil {
    out.LogString(fmt.Sprintf("RemoveEventLogSource() failed: %s", err))
    return fmt.Errorf("RemoveEventLogSource() failed: %s", err)
  }
  out.LogString("Fin removeService")
  return nil
}

func startService(name string) error {
  out.LogString(fmt.Sprintf("Inicio startService, servicio %s", name))
  m, err := mgr.Connect()
  if err != nil {
    out.LogString(fmt.Sprintf("could not connect to service manager: %v", err))
    return err
  }
  defer m.Disconnect()
  s, err := m.OpenService(name)
  if err != nil {
    out.LogString(fmt.Sprintf("could not access service: %v", err))
    return fmt.Errorf("could not access service: %v", err)
  }
  defer s.Close()
  err = s.Start()
  if err != nil {
    out.LogString(fmt.Sprintf("could not start service: %v", err))
    return fmt.Errorf("could not start service: %v", err)
  }
  out.LogString("Fin startService")
  return nil
}

func controlService(name string, c svc.Cmd, to svc.State) error {
  out.LogString(fmt.Sprintf("Inicio controlService, comando %d", c))
  m, err := mgr.Connect()
  if err != nil {
    out.LogString(fmt.Sprintf("could not connect to service manager: %v", err))
    return err
  }
  defer m.Disconnect()
  s, err := m.OpenService(name)
  if err != nil {
    out.LogString(fmt.Sprintf("could not access service: %v", err))
    return fmt.Errorf("could not access service: %v", err)
  }
  defer s.Close()
  status, err := s.Control(c)
  if err != nil {
    out.LogString(fmt.Sprintf("could not send control=%d: %v", c, err))
    return fmt.Errorf("could not send control=%d: %v", c, err)
  }
  timeout := time.Now().Add(10 * time.Second)
  for status.State != to {
    if timeout.Before(time.Now()) {
	  out.LogString(fmt.Sprintf("timeout waiting for service to go to state=%d", to))
      return fmt.Errorf("timeout waiting for service to go to state=%d", to)
    }
    time.Sleep(300 * time.Millisecond)
    status, err = s.Query()
    if err != nil {
	  out.LogString(fmt.Sprintf("could not retrieve service status: %v", err))
      return fmt.Errorf("could not retrieve service status: %v", err)
    }
  }
  out.LogString("Fin controlService")
  return nil
}

func main() {
  out.LogString("Inicio main")
  isIntSess, err := svc.IsAnInteractiveSession()
  if err != nil {
    out.LogString(fmt.Sprintf("failed to determine if we are running in an interactive session: %v", err))
    log.Fatalf("failed to determine if we are running in an interactive session: %v", err)
  }
  if !isIntSess {
    out.LogString(fmt.Sprintf("Is not an interactive session"))
    print("Is not an interactive session")
    runService(svcName, false)
    return
  }

  if len(os.Args) < 2 {
    out.LogString(fmt.Sprintf("no command specified"))
    usage("no command specified")
  }
  wsDebug = false
  cmd := strings.ToLower(os.Args[1])  
  switch cmd {
    case "debug":
      wsDebug = true
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
    out.LogString(fmt.Sprintf("failed to %s %s: %v", cmd, svcName, err))
    log.Fatalf("failed to %s %s: %v", cmd, svcName, err)
  }
  out.LogString("Fin main")
  return
}