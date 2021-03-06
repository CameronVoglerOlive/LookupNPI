package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ldk "github.com/open-olive/loop-development-kit/ldk/go"
)

// Serve creates the new loop and tells the LDK to serve it
func Serve() error {
	logger := ldk.NewLogger("venddy-search-searchbar")
	loop, err := NewLoop(logger)
	if err != nil {
		return err
	}
	ldk.ServeLoopPlugin(logger, loop)
	return nil
}

// Loop is a structure for generating SideKick whispers
type Loop struct {
	ctx    context.Context
	cancel context.CancelFunc

	sidekick ldk.Sidekick
	logger   *ldk.Logger
}

// NewLoop returns a pointer to a loop
func NewLoop(logger *ldk.Logger) (*Loop, error) {
	return &Loop{
		logger: logger,
	}, nil
}

var limit int = 20
var skip int = 0
var searchParams SearchParams

// LoopStart is called by the host when the loop is started to provide access to the host process
func (l *Loop) LoopStart(sidekick ldk.Sidekick) error {
	l.logger.Info("starting LookupNPI")
	l.ctx, l.cancel = context.WithCancel(context.Background())

	l.sidekick = sidekick

	return sidekick.UI().ListenSearchbar(l.ctx, func(text string, err error) {
		l.logger.Info("loop callback called")
		if err != nil {
			l.logger.Error("received error from callback", err)
			return
		}

		if text == "NPI" || text == "npi" {
			go l.run()
		}
	})
}

func (l *Loop) run() {
	ClearSearchParams()
	isSubmitted, _, err := l.sidekick.Whisper().Form(l.ctx, &ldk.WhisperContentForm{
		Label:       "NPI Lookup",
		Markdown:    "Enter Search Criteria",
		CancelLabel: "Cancel",
		SubmitLabel: "Search",
		Inputs:      l.CreateFormInputs(),
	})
	if err != nil {
		l.logger.Error("Form Whisper failed", "error", err)
	}

	lookupResults := l.GetLookupResults(RemoveSpaces(searchParams), limit, 0)

	elements := l.CreateDisambiguationElements(lookupResults.Results)

	if isSubmitted == true {
		_, err = l.sidekick.Whisper().Disambiguation(l.ctx, &ldk.WhisperContentDisambiguation{
			Label: fmt.Sprintf("LookupNPI Results for: %v %v %v %v %v %v",
				searchParams.Number,
				searchParams.FirstName,
				searchParams.LastName,
				searchParams.Organization,
				searchParams.City,
				searchParams.State),
			Elements: elements,
		})
	}
	ClearSearchParams()
}

func (l *Loop) CreateFormInputs() map[string]ldk.WhisperContentFormInput {
	inputs := map[string]ldk.WhisperContentFormInput{}
	inputs["number"] = &ldk.WhisperContentFormInputText{
		Label:   "NPI Number",
		Tooltip: "Exactly 10 digits",
		Order:   1,
		OnChange: func(number string) {
			if len(number) != 10 && len(number) > 0 {
				ctx, _ := context.WithTimeout(l.ctx, 5*time.Second)
				err := l.sidekick.Whisper().Markdown(ctx, &ldk.WhisperContentMarkdown{
					Label:    "LookupNPI Error",
					Markdown: "Npi must be 10 digits",
				})
				if err != nil {
					l.logger.Error("failed to emit whisper", "error", err)
					return
				}
			}
			searchParams.Number = number
		},
	}
	inputs["firstName"] = &ldk.WhisperContentFormInputText{
		Label:   "First Name",
		Tooltip: "Two Character Minimum",
		Order:   2,
		OnChange: func(firstName string) {
			searchParams.FirstName = firstName + "*"
		},
	}
	inputs["lastName"] = &ldk.WhisperContentFormInputText{
		Label:   "Last Name",
		Tooltip: "Two Character Minimum",
		Order:   3,
		OnChange: func(lastName string) {
			searchParams.LastName = lastName + "*"
		},
	}
	inputs["organization"] = &ldk.WhisperContentFormInputText{
		Label:   "Organization",
		Tooltip: "Two Character Minimum",
		Order:   4,
		OnChange: func(organization string) {
			searchParams.Organization = organization + "*"
		},
	}
	inputs["city"] = &ldk.WhisperContentFormInputText{
		Label:   "City",
		Tooltip: "Two Character Minimum",
		Order:   5,
		OnChange: func(city string) {
			searchParams.City = city + "*"
			if searchParams.State == "" {
				ctx, _ := context.WithTimeout(l.ctx, 5*time.Second)
				err := l.sidekick.Whisper().Markdown(ctx, &ldk.WhisperContentMarkdown{
					Label:    "LookNPI Error",
					Markdown: "Searching by City requires State",
				})
				if err != nil {
					l.logger.Error("failed to emit whisper", "error", err)
					return
				}
			}
		},
	}
	inputs["state"] = &ldk.WhisperContentFormInputText{
		Label:   "State",
		Tooltip: "2 Characters (Other criteria required)",
		Order:   6,
		OnChange: func(state string) {
			searchParams.State = state + "*"
			if searchParams.City == "" {
				ctx, _ := context.WithTimeout(l.ctx, 5*time.Second)
				err := l.sidekick.Whisper().Markdown(ctx, &ldk.WhisperContentMarkdown{
					Label:    "LookNPI Error",
					Markdown: "Searching by State requires City",
				})
				if err != nil {
					l.logger.Error("failed to emit whisper", "error", err)
					return
				}
			}
		},
	}
	return inputs
}

func RemoveSpaces(params SearchParams) SearchParams {
	params.City = strings.ReplaceAll(params.City, " ", "+")
	params.FirstName = strings.ReplaceAll(params.FirstName, " ", "+")
	params.LastName = strings.ReplaceAll(params.LastName, " ", "+")
	params.Number = strings.ReplaceAll(params.Number, " ", "+")
	params.Organization = strings.ReplaceAll(params.Organization, " ", "+")
	params.State = strings.ReplaceAll(params.State, " ", "+")
	return params
}

func ClearSearchParams() {
	searchParams.City = ""
	searchParams.FirstName = ""
	searchParams.LastName = ""
	searchParams.Number = ""
	searchParams.Organization = ""
	searchParams.State = ""
}

func (l *Loop) CreateDisambiguationElements(results []NpiInfo) map[string]ldk.WhisperContentDisambiguationElement {
	elements := make(map[string]ldk.WhisperContentDisambiguationElement)

	for i := range results {
		item := results[i]
		label := ""
		if item.Basic.FirstName != "" {
			label = fmt.Sprintf("Name: %v %v", item.Basic.FirstName, item.Basic.LastName)
		}
		if item.Basic.Organization != "" {
			label = fmt.Sprintf("Organization: %v", item.Basic.Organization)
		}
		elements[fmt.Sprintf("%v", i)] = &ldk.WhisperContentDisambiguationElementOption{
			Label: fmt.Sprintf("* NPI: %v, %v",
				item.Number,
				label),
			Order: uint32(i) + 1,
			OnChange: func(key string) {
				go func() {
					err := l.sidekick.Whisper().List(l.ctx, &ldk.WhisperContentList{
						Label:    fmt.Sprintf("Information for %v", item.Number),
						Elements: l.CreateListElements(item),
					})
					if err != nil {
						l.logger.Error("failed to emit whisper", "error", err)
					}
				}()
			},
		}
	}

	if len(results) == limit {
		elements["next"] = &ldk.WhisperContentDisambiguationElementOption{
			Label: "Next 20 Results",
			Order: uint32(len(results)) + 1,
			OnChange: func(key string) {
				go func() {
					skip += limit
					lookupResults := l.GetLookupResults(searchParams, limit, skip)

					elements := l.CreateDisambiguationElements(lookupResults.Results)

					_, err := l.sidekick.Whisper().Disambiguation(l.ctx, &ldk.WhisperContentDisambiguation{
						Label: fmt.Sprintf("LookupNPI Results for: %v %v %v %v %v %v",
							searchParams.Number,
							searchParams.FirstName,
							searchParams.LastName,
							searchParams.Organization,
							searchParams.City,
							searchParams.State),
						Elements: elements,
					})
					if err != nil {
						l.logger.Error("Whisper Disambiguation failed", "error", err)
					}
				}()
			},
		}
	}

	elements["prev"] = &ldk.WhisperContentDisambiguationElementOption{
		Label: "Previous 20 Results",
		Order: uint32(len(results)) + 1,
		OnChange: func(key string) {
			go func() {
				if skip != 0 {
					skip -= limit
				}
				lookupResults := l.GetLookupResults(searchParams, limit, skip)

				elements := l.CreateDisambiguationElements(lookupResults.Results)

				_, err := l.sidekick.Whisper().Disambiguation(l.ctx, &ldk.WhisperContentDisambiguation{
					Label: fmt.Sprintf("LookupNPI Results for: %v %v %v %v %v %v",
						searchParams.Number,
						searchParams.FirstName,
						searchParams.LastName,
						searchParams.Organization,
						searchParams.City,
						searchParams.State),
					Elements: elements,
				})
				if err != nil {
					l.logger.Error("Whisper Disambiguation failed", "error", err)
				}
			}()
		},
	}
	return elements
}

func (l *Loop) CreateListElements(item NpiInfo) map[string]ldk.WhisperContentListElement {
	elements := map[string]ldk.WhisperContentListElement{}
	enumerationName := ""
	switch item.EnumerationType {
	case "NPI-1":
		enumerationName = "Individual"
	case "NPI-2":
		enumerationName = "Organization"
	}
	elements["number"] = &ldk.WhisperContentListElementPair{
		Label: "NPI Number",
		Order: 1,
		Value: fmt.Sprintf("%v", item.Number),
	}
	elements["name"] = &ldk.WhisperContentListElementPair{
		Label: "Provider Name",
		Order: 2,
		Value: fmt.Sprintf("%v %v", item.Basic.FirstName, item.Basic.LastName),
	}
	elements["organization"] = &ldk.WhisperContentListElementPair{
		Label: "Organization",
		Order: 3,
		Value: fmt.Sprintf("%v", item.Basic.Organization),
	}
	elements["enumeration"] = &ldk.WhisperContentListElementPair{
		Label: "Enumeration Type",
		Order: 5,
		Value: fmt.Sprintf("%v", enumerationName),
	}
	elements["address"] = &ldk.WhisperContentListElementPair{
		Label: "Addresses",
		Order: 6,
		Value: CreateAddressStrings(item.Addresses),
	}
	elements["taxonomy"] = &ldk.WhisperContentListElementPair{
		Label: "Taxonomies",
		Order: 7,
		Value: CreateTaxonomyStrings(item.Taxonimies),
	}

	for i := range item.OtherNames {
		name := item.OtherNames[i]
		elements[fmt.Sprintf("%v", i)] = &ldk.WhisperContentListElementPair{
			Label: name.Type,
			Order: 4,
			Value: fmt.Sprintf("%v %v %v %v", name.Prefix, name.FirstName, name.LastName, name.Organization),
		}
	}

	return elements
}

func (l *Loop) GetLookupResults(searchParams SearchParams, limit int, skip int) LookupResults {
	var lookupResults LookupResults
	url := fmt.Sprintf("https://npiregistry.cms.hhs.gov/api/?number=%v&enumeration_type=&taxonomy_description=&first_name=%v&last_name=%v&organization_name=%v&address_purpose=&city=%v&state=%v&postal_code=&country_code=&limit=%v&skip=%v&version=2.0",
		searchParams.Number,
		searchParams.FirstName,
		searchParams.LastName,
		searchParams.Organization,
		searchParams.City,
		searchParams.State,
		limit,
		skip)
	resp, err := l.sidekick.Network().HTTPRequest(l.ctx, &ldk.HTTPRequest{
		URL:    url,
		Method: "GET",
		Body:   nil,
	})
	if err != nil {
		l.logger.Error("Lookup failed", err)
	}

	err = json.Unmarshal(resp.Data, &lookupResults)
	if err != nil {
		l.logger.Error("JSON Unmarshal failed", err)
	}
	return lookupResults
}

// LoopStop is called by the host when the loop is stopped
func (l *Loop) LoopStop() error {
	l.logger.Info("LoopStop called")
	l.cancel()

	return nil
}

func CreateTaxonomyStrings(taxonomies []Taxonomy) string {
	taxonomyString := ""
	for i := range taxonomies {
		tax := taxonomies[i]
		taxonomyString += fmt.Sprintf("%v) Primary:%v  Code:%v Description:%v State:%v License:%v\n", i+1, tax.Primary, tax.Code, tax.Description, tax.State, tax.License)
	}
	return taxonomyString
}

func CreateAddressStrings(addresses []Address) string {
	addressString := ""
	for i := range addresses {
		address := addresses[i]
		addressString += fmt.Sprintf("%v) %v, %v %v %v\n", i+1, address.City, address.State, address.PostalCode, address.Country)
	}
	return addressString
}

type LookupResults struct {
	ResultCount int       `json:"result_count"`
	Results     []NpiInfo `json:"results"`
}

type NpiInfo struct {
	Number          int         `json:"number"`
	Basic           Basic       `json:"basic"`
	EnumerationType string      `json:"enumeration_type"`
	Taxonimies      []Taxonomy  `json:"taxonomies"`
	Addresses       []Address   `json:"addresses"`
	OtherNames      []OtherName `json:"other_names"`
}

type Basic struct {
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Organization string `json:"organization_name"`
}

type OtherName struct {
	Type         string `json:"type"`
	Prefix       string `json:"prefix"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Organization string `json:"organization_name"`
}

type Address struct {
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country_name"`
}

type Taxonomy struct {
	Code        string `json:"code"`
	Description string `json:"desc"`
	Primary     bool   `json:"primary"`
	State       string `json:"state"`
	License     string `json:"license"`
}

type SearchParams struct {
	Number       string
	FirstName    string
	LastName     string
	City         string
	State        string
	Organization string
}
