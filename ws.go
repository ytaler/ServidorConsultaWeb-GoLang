package main

import (
  "net/http"
  //"strings"
  "fmt"
  "database/sql"
  "flag"
  "log"
  "go-mssqldb"
  "regexp"
  "net/url"
  "strconv"
  "time"
  //"encoding/base64"
  //"bytes"
)

var (
  debug         = flag.Bool("debug", false, "Habilitar depuracion")
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
  if *debug {
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
  if *debug {
    fmt.Printf("Consulta: %s\n",query)
  }
  // se prepara la query
  stmt, err := conn.Prepare(query)
  if err != nil {
    log.Fatal("Prepare failed:", err.Error())
    http.NotFound(w, r)
    fmt.Printf(" Error al preparar query SQL - Path=%s - bolilla=%d", r.URL.Path, *bolilla)
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
    log.Fatal("Scan failed:", err.Error())
    http.NotFound(w, r)
    fmt.Printf(" Error al parsear resultado query SQL - Path=%s - bolilla=%d", r.URL.Path, *bolilla)
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

  if *debug {
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
  	log.Fatal("Bad response:", err.Error())
    http.NotFound(w, r)
    fmt.Printf(" Error de comunicacion con PPress - Path=%s - bolilla=%d", r.URL.Path, *bolilla)
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
  if *debug {
    fmt.Printf("Hora finalizacion balance: %s\n",time.Now())
  }
}

func consultador(w http.ResponseWriter, r *http.Request) {
  //fmt.Fprintf(w, "<html><head><title>Servicio de consulta masiva de comprobantes</title></head></html>")
  if *debug {
    fmt.Printf("********************************************************************************\nHora inicio consulta: %s\n",time.Now())
  }

  consultaBase := url.Values{}
  // verificacion de path url, si no es corecto, retorna http 403
  m := pathConsulta.FindStringSubmatch(r.URL.Path)
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
  consultaBase.Set("existeComprobante",r.FormValue("e"))
  // se arma la variable que contiene la consulta
  query := "Exec ConsultaExiste '"+ consultaBase.Get("prefijoComprobante") +"','"+ consultaBase.Get("numeroComprobante") +"','"+ consultaBase.Get("cuentaComprobante") +"','" + consultaBase.Get("distribuidoraComprobante") + "'"
  if *debug {
    fmt.Printf("Consulta: %s\n",query)
  }
  // se prepara la query
  stmt, err := conn.Prepare(query)
  if err != nil {
    log.Fatal("Prepare failed:", err.Error())
    http.NotFound(w, r)
    fmt.Printf(" Error al preparar query SQL - Path=%s - bolilla=%d", r.URL.Path, *bolilla)
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
    log.Fatal("Scan failed:", err.Error())
    http.NotFound(w, r)
    fmt.Printf(" Error al parsear resultado query SQL - Path=%s - bolilla=%d", r.URL.Path, *bolilla)
    r.ParseForm()
    for key, values := range r.Form {
      fmt.Printf(" - %s: %s", key,values)
    }
    print("\n")
    return
  }

  if *debug {
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
  if *debug {
    fmt.Printf("Hora finalizacion consulta: %s\n",time.Now())
  }
}

func main() {

  flag.Parse()

  if *debug {
    fmt.Printf("********************************************************************************\n password:%s\n", *password)
    fmt.Printf(" port:%d\n", *port)
    fmt.Printf(" server:%s\n", *server)
    fmt.Printf(" user:%s\n", *user)
    fmt.Printf(" database:%s\n", *database)
    fmt.Printf(" bolilla:%s\n", *bolilla)
  }

  connString := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%d;database=%s", *server, *user, *password, *port, *database)
  if *debug {
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
}