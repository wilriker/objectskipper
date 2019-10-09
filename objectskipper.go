package main

import (
	"flag"
	"log"
	"sort"

	"strings"

	"github.com/wilriker/goduetapiclient/connection"
	"github.com/wilriker/goduetapiclient/connection/initmessages"
	"github.com/wilriker/goduetapiclient/types"
)

type settings struct {
	SocketPath              string
	ManageIdentifierPattern int64
	ManageObjectIds         int64
	CurrentObjectId         int64
	Disable                 int64
}

func main() {
	s := settings{}

	flag.StringVar(&s.SocketPath, "socketPath", connection.DefaultSocketPath, "Path to socket")
	flag.Int64Var(&s.ManageIdentifierPattern, "idpattern", 50, "M-number to manage object identifier patterns")
	flag.Int64Var(&s.ManageObjectIds, "objectid", 51, "M-number to manage object Ids")
	flag.Int64Var(&s.CurrentObjectId, "currentid", 52, "M-number to list/add current object Id")
	flag.Int64Var(&s.Disable, "disable", 53, "M-number to disable filtering")
	flag.Parse()

	ic := connection.InterceptConnection{}
	err := ic.Connect(initmessages.InterceptionModePre, s.SocketPath)
	if err != nil {
		panic(err)
	}
	defer ic.Close()

	filter(ic, s)
}

func getSortedKeys(m map[string]bool) []string {
	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

func filter(ic connection.InterceptConnection, s settings) {
	filtering := false
	currentObjectId := ""
	patterns := make(map[string]bool)
	objectsToFilter := make(map[string]bool)
	knownObjectIds := make(map[string]bool)
	for {
		c, err := ic.ReceiveCode()
		if err != nil {
			log.Println("Error:", err)
			continue
		}
		if c.Type == types.MCode && c.MajorNumber != nil {
			switch *c.MajorNumber {
			case s.ManageIdentifierPattern:
				p := c.ParameterOrDefault("P", "").AsString()
				if p == "" {
					var m string
					if len(patterns) == 0 {
						m = "No identifier patterns defined."
					} else {
						m = "Active identifier patterns: " + strings.Join(getSortedKeys(patterns), ", ")
					}
					err = ic.ResolveCode(types.Success, m)
					break
				}
				sParam := c.ParameterOrDefault("S", 1)
				sVal, _ := sParam.AsUint64()
				patterns[p] = sVal == 1
				err = ic.ResolveCode(types.Success, "")

			case s.ManageObjectIds:
				o := c.ParameterOrDefault("P", "").AsString()
				if o == "" {
					var m string
					if len(objectsToFilter) == 0 {
						m = "No objectIds defined."
					} else {
						m = "Current active objectIds: " + strings.Join(getSortedKeys(objectsToFilter), ", ")
					}
					err = ic.ResolveCode(types.Success, m)
					break
				}
				sParam := c.ParameterOrDefault("S", 1)
				sVal, _ := sParam.AsUint64()
				objectsToFilter[o] = sVal == 1
				err = ic.ResolveCode(types.Success, "")

			case s.CurrentObjectId:
				sParam := c.ParameterOrDefault("S", 0)
				sVal, _ := sParam.AsUint64()
				var b strings.Builder
				switch sVal {
				case 0:
					b.WriteString("Current objectId")
					if currentObjectId != "" {
						b.WriteString(":")
						b.WriteString(currentObjectId)
						b.WriteString(".")
					} else {
						b.WriteString(" unknown.")
					}
					if len(knownObjectIds) > 0 {
						b.WriteString(" Known objectIds: ")
						keys := getSortedKeys(knownObjectIds)
						b.WriteString(strings.Join(keys, ", "))
					}
				case 2:
					filtering = true
					fallthrough
				case 1:
					objectsToFilter[currentObjectId] = true
				}
				err = ic.ResolveCode(types.Success, b.String())

			case s.Disable:
				filtering = false
				var reset string
				if sParam, _ := c.ParameterOrDefault("S", 0).AsUint64(); sParam == 1 {
					patterns = make(map[string]bool)
					objectsToFilter = make(map[string]bool)
					reset = ", all patterns and objectIds deleted."
				}
				err = ic.ResolveCode(types.Success, "Filtering disabled"+reset)

			default:
				if filtering {
					err = ic.ResolveCode(types.Success, "")
				} else {
					err = ic.IgnoreCode()
				}
			}
		} else if c.Type == types.Comment && c.Comment != "" {
			comment := strings.TrimSpace(c.Comment)
			for p, _ := range patterns {
				if strings.HasPrefix(comment, p) {
					currentObjectId = strings.TrimSpace(strings.TrimLeft(comment, p))
					knownObjectIds[currentObjectId] = true
					filtering = objectsToFilter[currentObjectId]
					break
				}
			}
			err = ic.IgnoreCode()
		} else if filtering {
			err = ic.ResolveCode(types.Success, "")
		} else {
			err = ic.IgnoreCode()
		}
		if err != nil {
			log.Println("Error:", err)
			err = nil
		}
	}
}
