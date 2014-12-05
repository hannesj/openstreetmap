
package main

import "encoding/json"
import "fmt"
import "bytes"
import "os"
import "log"
import "io"
import "time"
import "runtime"
import "strings"
import "strconv"
import "github.com/qedus/osmpbf"
import "github.com/syndtr/goleveldb/leveldb"

func main() {

  file := openFile()
  defer file.Close()

  decoder := osmpbf.NewDecoder(file)
  err := decoder.Start(runtime.GOMAXPROCS(-1)) // use several goroutines for faster decoding
  if err != nil {
    log.Fatal(err)
  }

  db := openLevelDB("/tmp/myleveldb")
  defer db.Close()

  run(decoder, db)
}

func run(d *osmpbf.Decoder, db *leveldb.DB){

  features := getFeatures()

  var nc, wc, rc uint64
  for {
    if v, err := d.Decode(); err == io.EOF {
      break
    } else if err != nil {
      log.Fatal(err)
    } else {
      switch v := v.(type) {

        case *osmpbf.Node:

          // write to leveldb
          cacheStore(db, v)

          if !hasTags(v.Tags) { break }
          if isAddress(v.Tags) || isInFeatureList(v.Tags,features) {
            onNode(v)
          }
          nc++
        
        case *osmpbf.Way:

          if !hasTags(v.Tags) { break }
          if isAddress(v.Tags) || isInFeatureList(v.Tags,features) {

            // lookup from leveldb
            latlons := cacheLookup(db, v)

            onWay(v,latlons)
          }
          wc++

        case *osmpbf.Relation:
          
          onRelation(v)
          rc++

        default:

          log.Fatalf("unknown type %T\n", v)

      }
    }
  }

  // fmt.Printf("Nodes: %d, Ways: %d, Relations: %d\n", nc, wc, rc)
}

type JsonNode struct {
  ID        int64               `json:"id"`
  Type      string              `json:"type"`
  Lat       float64             `json:"lat"`
  Lon       float64             `json:"lon"`
  Tags      map[string]string   `json:"tags"`
  Timestamp time.Time           `json:"timestamp"`
}

func onNode(node *osmpbf.Node){
  marshall := JsonNode{ node.ID, "node", node.Lat, node.Lon, node.Tags, node.Timestamp }
  json, _ := json.Marshal(marshall)
  fmt.Println(string(json))
}

type JsonWay struct {
  ID        int64               `json:"id"`
  Type      string              `json:"type"`
  Tags      map[string]string   `json:"tags"`
  // NodeIDs   []int64             `json:"refs"`
  Nodes     []map[string]string `json:"nodes"`
  Timestamp time.Time           `json:"timestamp"`
}

func onWay(way *osmpbf.Way, latlons []map[string]string){
  marshall := JsonWay{ way.ID, "way", way.Tags/*, way.NodeIDs*/, latlons, way.Timestamp }
  json, _ := json.Marshal(marshall)
  fmt.Println(string(json))
}

func onRelation(relation *osmpbf.Relation){
  // do nothing (yet)
}

func cacheStore(db *leveldb.DB, node *osmpbf.Node){
  id, val := formatLevelDB(node)
  err := db.Put([]byte(id), []byte(val), nil)
  if err != nil {
    log.Fatal(err)
  }
}

func cacheLookup(db *leveldb.DB, way *osmpbf.Way) []map[string]string{

  var container []map[string]string

  for _, each := range way.NodeIDs {
    stringid := strconv.FormatInt(each,10)

    data, err := db.Get([]byte(stringid), nil)
    if err != nil {
      log.Fatal(err)
    }

    s := string(data)
    spl := strings.Split(s, ":");

    latlon := make(map[string]string)
    lat, lon := spl[0], spl[1]
    latlon["lat"] = lat
    latlon["lon"] = lon

    container = append(container, latlon)

  }

  return container

  // fmt.Println(way.NodeIDs)
  // fmt.Println(container)
  // os.Exit(1)
}

func formatLevelDB(node *osmpbf.Node) (id string, val []byte){

  stringid := strconv.FormatInt(node.ID,10)

  var bufval bytes.Buffer
  bufval.WriteString(strconv.FormatFloat(node.Lat,'f',6,64))
  bufval.WriteString(":")
  bufval.WriteString(strconv.FormatFloat(node.Lon,'f',6,64))
  byteval := []byte(bufval.String())
  
  return stringid, byteval
}

func openFile() *os.File {
  // no file specified
  if len(os.Args) < 2 {
    log.Fatal("invalid file: you must specify a pbf path as arg[1]")
  }
  // try to open the file
  file, err := os.Open(os.Args[1])
  if err != nil {
    log.Fatal(err)
  }  
  return file
}

func openLevelDB(path string) *leveldb.DB {
  // try to open the db
  db, err := leveldb.OpenFile(path, nil)
  if err != nil {
    log.Fatal(err)
  }
  return db
}

// extract all keys to array
// keys := []string{}
// for k := range v.Tags {
//     keys = append(keys, k)
// }

func hasTags(tags map[string]string) bool {
  n := len(tags)
  if n == 0 {
    return false
  }
  return true
}

func isAddress(tags map[string]string) bool {
  _, test1 := tags["addr:housenumber"]
  _, test2 := tags["addr:street"]
  return test1 && test2
}

func isInFeatureList(tags map[string]string, features []string) bool {
  for _, each := range features {
    _, test := tags[each]
    if test {
      return true
    }
  }
  return false
}

func getFeatures() []string{
  features := []string{
    "amenity",
    "building",
    "shop",
    "office",
    "public_transport",
    "cuisine",
    "railway",
    "sport",
    "natural",
    "tourism",
    "leisure",
    "historic",
    "man_made",
    "landuse",
    "waterway",
    "aerialway",
    "aeroway",
    "craft",
    "military",
  }
  return features
}