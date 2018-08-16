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

// http://localhost:8082/balance/balance.php?d=32541040&p=0001&c=12345678&t=cen&o=si
// http://192.168.1.25:8082/balance/balance.php?d=32541040&p=0001&c=12345678&t=cen&o=si

// Consulta PPRESS
// http://192.168.1.230:8081/prueba?nombre=Cordobesa.pdf&pagina=300&periodo=2018\08\&ruta=PDF\&servidor=\\192.168.1.230\

var (
  debug         = flag.Bool("debug", false, "Habilitar depuracion")
  password      = flag.String("password", "Doxer2018.,", "DB password")
  port     *int = flag.Int("port", 1433, "DB port")
  server        = flag.String("server", "192.168.1.250\\SQLDOXER", "DB server")
  user          = flag.String("user", "sa", "DB user")
  database      = flag.String("database", "Dx_Indices", "DB name")
  bolilla  *int = flag.Int("bollila", 1, "Bolilla inicial")
  conn *sql.DB
  err error
  servidorPPress= "http://192.168.1.230:8081/prueba"
  puertoHttp    = ":8082"
)

var pathBalance = regexp.MustCompile("^/(balance/balance.php)$")
var pathConsulta = regexp.MustCompile("^/(consulta/consulta.php)$")

// * Extracción de comprobante:
// http://10.2.1.221:8082/balance/balance.php?d=00297381&p=0210&c=140692&t=cen&o=si
// http://192.168.1.25:8082/balance/balance.php?d=32541040&p=0001&c=12345678&t=cen&o=si
// d --> cuenta
// p --> prefijo
// c --> cuenta
// t --> distribuidora: cen/cuy
// o --> observaciones: nof=no lleva fondo, resto con fondo.

func balanceador(w http.ResponseWriter, r *http.Request) {
  //fmt.Fprintf(w, "<html><head><title>Servicio de consulta masiva de comprobantes</title></head></html>")
  if *debug {
    fmt.Printf("****************************************************************************************************\nHora inicio balance: %s\n",time.Now())
  }

  consultaBase := url.Values{}
  resultadoBase := url.Values{}
  // verificacion de path url, si no es corecto, retorna http 403
  m := pathBalance.FindStringSubmatch(r.URL.Path)
  //print(r.URL.Path)
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

  consultaBase.Set("numeroComprobante",r.FormValue("d"))
  consultaBase.Set("prefijoComprobante",r.FormValue("p"))
  consultaBase.Set("cuentaComprobante",r.FormValue("c"))
  if r.FormValue("t") == "cen"{
  	consultaBase.Set("distribuidoraComprobante","C")
  }else{
  	consultaBase.Set("distribuidoraComprobante","Y")
  }
  consultaBase.Set("observacionesComprobante",r.FormValue("o"))

  // COLUMNAS BASE
  // servidor,ruta,distribuidora,preriodo,nombre_archivo,prefijo_comp,numero_comp,tipo_comp,fomulario,numero_cta,pagina,time_factura
  //query := "select servidor, ruta, preriodo, nombre_archivo, pagina from Indices where prefijo_comp='"+ consultaBase.Get("prefijoComprobante") +"' and numero_comp='"+ consultaBase.Get("numeroComprobante") +"' and distribuidora='" + consultaBase.Get("distribuidoraComprobante") + "'"
  query := "Exec ExtraePDF '" + consultaBase.Get("prefijoComprobante") + "','" + consultaBase.Get("numeroComprobante") + "','" + consultaBase.Get("cuentaComprobante") + "','" + consultaBase.Get("distribuidoraComprobante") + "'"
  if *debug {
    fmt.Printf("Consulta: %s\n",query)
  }
  stmt, err := conn.Prepare(query)
  if err != nil {
    //log.Fatal("Prepare failed:", err.Error())
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

  row := stmt.QueryRow()

  var servidor string
  var ruta string
  var distribuidora string
  var periodo string
  var nombre string
  var prefijo_comp string
  var numero_comp string
  var tipo_comp string
  var fomulario string
  var numero_cta string
  var pagina int
  var time_factura string
  
  err = row.Scan(&servidor, &ruta, &distribuidora, &periodo, &nombre, &prefijo_comp, &numero_comp, &tipo_comp, &fomulario, &numero_cta, &pagina, &time_factura)
  if err != nil {
    //log.Fatal("Scan failed:", err.Error())
    http.NotFound(w, r)
    fmt.Printf(" Error al parsear resultado query SQL - Path=%s - bolilla=%d", r.URL.Path, *bolilla)
    r.ParseForm()
    for key, values := range r.Form {
      fmt.Printf(" - %s: %s", key,values)
    }
    print("\n")
    return
  }

  resultadoBase.Set("servidor",servidor)
  resultadoBase.Set("ruta",ruta)
  resultadoBase.Set("periodo",periodo)
  resultadoBase.Set("nombre",nombre)
  resultadoBase.Set("pagina",strconv.Itoa(pagina))

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
  resp, err := http.PostForm(servidorPPress, resultadoBase)

  if resp.StatusCode != 200{
    http.NotFound(w, r)
    fmt.Printf(" Error de comunicacion con PPress - Path=%s - bolilla=%d", r.URL.Path, *bolilla)
    r.ParseForm()
    for key, values := range r.Form {
      fmt.Printf(" - %s: %s", key,values)
    }
    print("\n")
    return
  }

  // FORMATO DE NOMBRE
  // fact-ecogas-pppp-DDDDDDDD-CCCCCCCC-xxx.pdf
  // pppp es punto de venta (prefijo)
  // DDDDDDDD es numero de factura (numero)
  // CCCCCCCC es numero de cuenta
  // xxxx es cen o cuy sin comillas

  nombreFinal := fmt.Sprintf("fact-ecogas-%s-%s-%s-%s",consultaBase.Get("prefijoComprobante"),consultaBase.Get("numeroComprobante"),consultaBase.Get("cuentaComprobante"),consultaBase.Get("distribuidoraComprobante"))
  //w.Header().Set("Content-Disposition", "attachment; filename="+resultadoBase.Get("nombre"))
  w.Header().Set("Content-Disposition", "inline; filename="+nombreFinal)
  w.Header().Set("Content-Type", "application/pdf")
  resp.Write(w)
  defer resp.Body.Close()
  //http.ServeFile(w, r, &resultadoBase.Get("servidor")+resultadoBase.Get("ruta")+resultadoBase.Get("periodo")+resultadoBase.Get("nombre"))
  //http.ServeFile(w, r, resp.Request)  
  //pdf, err := base64.StdEncoding.DecodeString("aca va el pdf codificado en base64")
  //http.ServeContent(w, r, resultadoBase.Get("nombre"), time.Now(), strings.NewReader(string(pdf)))  
  //http.ServeContent(w, r, resultadoBase.Get("nombre"), time.Now(), bytes.NewReader(pdf))
  //w.Write([]byte(message))
  *bolilla += 1
  if (*bolilla >= 5)||(*bolilla < 1){
    *bolilla = 1
  }
  if *debug {
    fmt.Printf("Hora finalizacion balance: %s\n",time.Now())
  }
}

// * Consulta de existencia de comprobante
// http://10.2.1.221:8082/balance/consulta.php?p=0210&d=00297381&c=140692&t=cen&e=1
// http://192.168.1.25:8082/consulta/consulta.php?d=32541040&p=0001&c=12345678&t=cen&e=1
// d --> cuenta
// p --> prefijo
// c --> cuenta
// t --> distribuidora: cen/cuy
// e --> existe: 1/0

func consultador(w http.ResponseWriter, r *http.Request) {
  //fmt.Fprintf(w, "<html><head><title>Servicio de consulta masiva de comprobantes</title></head></html>")
  if *debug {
    fmt.Printf("****************************************************************************************************\nHora inicio consulta: %s\n",time.Now())
  }

  consultaBase := url.Values{}

  // verificacion de path url, si no es corecto, retorna http 403
  m := pathConsulta.FindStringSubmatch(r.URL.Path)
  //print(r.URL.Path)
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

  consultaBase.Set("numeroComprobante",r.FormValue("d"))
  consultaBase.Set("prefijoComprobante",r.FormValue("p"))
  consultaBase.Set("cuentaComprobante",r.FormValue("c"))
  if r.FormValue("t") == "cen"{
  	consultaBase.Set("distribuidoraComprobante","C")
  }else{
  	consultaBase.Set("distribuidoraComprobante","Y")
  }
  consultaBase.Set("existeComprobante",r.FormValue("e"))

  // COLUMNAS BASE
  // servidor,ruta,distribuidora,preriodo,nombre_archivo,prefijo_comp,numero_comp,tipo_comp,fomulario,numero_cta,pagina,time_factura
  query := "Exec ConsultaExiste '"+ consultaBase.Get("prefijoComprobante") +"','"+ consultaBase.Get("numeroComprobante") +"','"+ consultaBase.Get("cuentaComprobante") +"','" + consultaBase.Get("distribuidoraComprobante") + "'"
  if *debug {
    fmt.Printf("Consulta: %s\n",query)
  }
  stmt, err := conn.Prepare(query)
  if err != nil {
    //log.Fatal("Prepare failed:", err.Error())
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

  row := stmt.QueryRow()

  var respuesta string
  
  err = row.Scan(&respuesta)
  if err != nil {
    //log.Fatal("Scan failed:", err.Error())
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

  //w.Write([]byte("<html><head><title>Sistema de consulta</title></head><body><h1>HOLA QUE TAL</h1></body></html>"))
  w.Write([]byte(respuesta))
  if *debug {
    fmt.Printf("Hora finalizacion consulta: %s\n",time.Now())
  }

}

func main() {

  flag.Parse()

  if *debug {
    fmt.Printf("****************************************************************************************************\n password:%s\n", *password)
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