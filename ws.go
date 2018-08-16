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

// te dirijo a 10.0.0.1!
// Error: "Failed to connect to 192.168.1.37 port 8081: Connection refused" - Code: 7
// http://192.168.1.37:8080/balance/balance.php?consultaBase.Get("numeroComprobante")=14265747&consultaBase.Get("prefijoComprobante")=0211&consultaBase.Get("tipoComprobante")=B

// http://localhost:8080/balance/balance?consultaBase.Get("numeroComprobante")=32541040&consultaBase.Get("prefijoComprobante")=0001&consultaBase.Get("tipoComprobante")=A

// http://localhost:8080/balance/balance?consultaBase.Get("numeroComprobante")=00033824&consultaBase.Get("prefijoComprobante")=0216&consultaBase.Get("tipoComprobante")=A

// Consulta PPRESS
// http://192.168.1.230:8081/prueba?nombre=Cordobesa.pdf&pagina=300&periodo=2018\08\&ruta=PDF\&servidor=\\192.168.1.230\

var (
  debug         = flag.Bool("debug", false, "Habilitar depuracion")
  password      = flag.String("password", "Doxer2018.,", "DB password")
  port     *int = flag.Int("port", 1433, "DB port")
  server        = flag.String("server", "192.168.1.250\\SQLDOXER", "DB server")
  user          = flag.String("user", "sa", "DB user")
  database      = flag.String("database", "Eco-2018", "DB name")
  bolilla  *int = flag.Int("bollila", 1, "Bolilla inicial")
  conn *sql.DB
  err error
  servidorPPress= "http://192.168.1.230:8081/prueba"
  puertoHttp    = ":8080"
)

var validPath = regexp.MustCompile("^/(balance/balance)$")

func balanceador(w http.ResponseWriter, r *http.Request) {
  //fmt.Fprintf(w, "<html><head><title>Servicio de consulta masiva de comprobantes</title></head></html>")
  if *debug {
    fmt.Printf("%s\n",time.Now())
  }

  consultaBase := url.Values{}
  resultadoBase := url.Values{}
  // verificacion de path url, si no es corecto, retorna http 403
  m := validPath.FindStringSubmatch(r.URL.Path)
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

  consultaBase.Set("numeroComprobante",r.FormValue("num_comp"))
  consultaBase.Set("prefijoComprobante",r.FormValue("pre_comp"))
  consultaBase.Set("tipoComprobante",r.FormValue("tip_comp"))

  //debug := "Parametros recibidos: " + consultaBase.Get("numeroComprobante") + " " + consultaBase.Get("prefijoComprobante") + " " + consultaBase.Get("tipoComprobante") + " - Bolilla " + fmt.Sprintf("%d",bolilla)

  // COLUMNAS BASE
  // servidor,ruta,distribuidora,preriodo,nombre_archivo,prefijo_comp,numero_comp,tipo_comp,fomulario,numero_cta,pagina,time_factura
  query := "select servidor, ruta, distribuidora, preriodo, nombre_archivo, numero_cta, pagina from Indices where prefijo_comp='"+ consultaBase.Get("prefijoComprobante") +"' and numero_comp='"+ consultaBase.Get("numeroComprobante") +"' and tipo_comp='" + consultaBase.Get("tipoComprobante") + "'"
  //query := "select servidor, ruta, preriodo, nombre_archivo from Indices where prefijo_comp='0216' and numero_comp='00033824' and tipo_comp='A'"
  //print(query)
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
  var cuenta string
  var pagina int
  
  err = row.Scan(&servidor, &ruta, &distribuidora, &periodo, &nombre, &cuenta, &pagina)
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
  if distribuidora == "C"{
  	resultadoBase.Set("distribuidora","cen")
  }else{
	resultadoBase.Set("distribuidora","cuy")
  }
  resultadoBase.Set("periodo",periodo)
  resultadoBase.Set("nombre",nombre)
  resultadoBase.Set("cuenta",cuenta)
  resultadoBase.Set("pagina",strconv.Itoa(pagina))

  if *debug {
    fmt.Printf("Archivo: %s%s%s%s Pagina:%s", resultadoBase.Get("servidor"), resultadoBase.Get("ruta"), resultadoBase.Get("periodo"), resultadoBase.Get("nombre"), resultadoBase.Get("pagina"))
    fmt.Printf(" tipo=%s, prefijo=%s, numero=%s, bolilla=%d\n", consultaBase.Get("tipoComprobante"), consultaBase.Get("prefijoComprobante"), consultaBase.Get("numeroComprobante"), *bolilla)
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

  nombreFinal := fmt.Sprintf("fact-ecogas-%s-%s-%s-%s",consultaBase.Get("prefijoComprobante"),consultaBase.Get("numeroComprobante"),resultadoBase.Get("cuenta"),resultadoBase.Get("distribuidora"))
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
    fmt.Printf("%s\n",time.Now())
  }
}

func main() {

  flag.Parse()

  if *debug {
    fmt.Printf(" password:%s\n", *password)
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
  if err := http.ListenAndServe(puertoHttp, nil); err != nil {
    panic(err)
  }
}