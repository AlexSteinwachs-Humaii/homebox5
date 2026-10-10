package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

type ReReplace struct {
	Regex *regexp.Regexp
	Text  string
}

func NewReReplace(regex string, replace string) ReReplace {
	return ReReplace{
		Regex: regexp.MustCompile(regex),
		Text:  replace,
	}
}

func NewReDate(dateStr string) ReReplace {
	return ReReplace{
		Regex: regexp.MustCompile(fmt.Sprintf(`%s: string`, dateStr)),
		Text:  fmt.Sprintf(`%s: Date | string`, dateStr),
	}
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Please provide a file path as an argument")
		os.Exit(1)
	}

	path := os.Args[1]

	fmt.Printf("Processing %s\n", path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("File %s does not exist\n", path)
		os.Exit(1)
	}

	text := "/* post-processed by backend/app/tools/typegen/main.go */\n"
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	text += string(data)

	replaces := [...]ReReplace{
		NewReReplace(` Repo`, " "),
		NewReReplace(` PaginationResultRepo`, " PaginationResult"),
		NewReReplace(` Services`, " "),
		NewReReplace(` V1`, " "),
		NewReReplace(`\?:`, ":"),
		NewReReplace(`(\w+):\s(.*null.*)`, "$1?: $2"), // make null union types optional
		NewReDate("createdAt"),
		NewReDate("updatedAt"),
		NewReDate("soldDate"),
		NewReDate("purchaseDate"),
		NewReDate("warrantyExpires"),
		NewReDate("expiresAt"),
		NewReDate("date"),
		NewReDate("completedDate"),
		NewReDate("scheduledDate"),
	}

	text = processContracts(text, replaces[:])

	err = os.WriteFile(path, []byte(text), 0644)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	os.Exit(0)
}

// CSV contracts retain Swagger optionality and wire-format timestamp strings.
// Legacy consumers still receive the historical transformations outside CSV.
func processContracts(text string, replaces []ReReplace) string {
	csv := regexp.MustCompile(`(?s)export (?:interface|type) (?:ServicesCSV|V1CSV|V1ResultsServicesCSV)[^\n]*?\{.*?\n\}`)
	var protected []string
	text = csv.ReplaceAllStringFunc(text, func(block string) string {
		for _, replace := range replaces {
			if replace.Regex.String() == ` Services` || replace.Regex.String() == ` V1` {
				block = replace.Regex.ReplaceAllString(block, replace.Text)
			}
		}
		protected = append(protected, block)
		return fmt.Sprintf("__CSV_CONTRACT_%d__", len(protected)-1)
	})
	for _, replace := range replaces {
		text = replace.Regex.ReplaceAllString(text, replace.Text)
	}
	for i, block := range protected {
		text = strings.ReplaceAll(text, fmt.Sprintf("__CSV_CONTRACT_%d__", i), block)
	}
	return text
}
