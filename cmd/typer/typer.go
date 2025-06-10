package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/dave/jennifer/jen"
	"github.com/go-vgo/robotgo"
	"pkg.jsn.cam/jsn/internal"

	_ "github.com/go-vgo/robotgo/base"  // Blank import for robotgo C sources
	_ "github.com/go-vgo/robotgo/key"   // Blank import for robotgo C sources
	_ "github.com/go-vgo/robotgo/mouse" // Blank import for robotgo C sources
)

// initLogger initializes the logging system to output to the console.
func initLogger() {
	logMessage("Logging system initialized (console output only).")
}

// logMessage writes a message to the console with a timestamp.
func logMessage(v ...any) {
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	fmt.Printf("%s: %s\n", timestamp, fmt.Sprint(v...))
}

// goKeywords is a list of Go keywords and common identifiers used for code generation.
var goKeywords = []string{
	"break", "default", "func", "interface", "select", "case", "defer", "go", "map", "struct",
	"chan", "else", "goto", "package", "switch", "const", "fallthrough", "if", "range", "type",
	"continue", "for", "import", "return", "var", "append", "bool", "byte", "cap", "close",
	"complex", "complex64", "complex128", "uint16", "uint32", "uint64", "int8", "int16",
	"int32", "int64", "float32", "float64", "uintptr", "true", "false", "iota", "nil",
	"string", "int", "uint", "len", "make", "new", "panic", "print", "println", "real",
	"recover", "error", "context", "http", "fmt", "os", "io", "json", "time", "rand",
	"math", "sort", "sync", "strconv", "strings", "bytes", "log", "net", "sql", "regexp",
	"err", "ctx", "req", "res", "data", "result", "val", "key", "idx", "item", "user",
	"id", "name", "config", "server", "client", "response", "request", "wg", "mu", "ch",
	"done", "quit", "stop", "handle", "query", "route", "model", "util", "helper", "service",
}

// nearbyKeys maps characters to a list of keys physically close on a QWERTY keyboard.
// Used for simulating typos.
var nearbyKeys = map[rune][]rune{
	'q': {'w', 'a', 's'}, 'w': {'q', 'e', 's', 'd', 'a'}, 'e': {'w', 'r', 'd', 'f', 's'}, 'r': {'e', 't', 'f', 'g', 'd'},
	't': {'r', 'y', 'g', 'h', 'f'}, 'y': {'t', 'u', 'h', 'j', 'g'}, 'u': {'y', 'i', 'j', 'k', 'h'}, 'i': {'u', 'o', 'k', 'l', 'j'},
	'o': {'i', 'p', 'l', ';', 'k'}, 'p': {'o', '[', ';', ':', 'l'}, 'a': {'q', 'w', 's', 'z', 'x'}, 's': {'a', 'd', 'w', 'e', 'x', 'z', 'c'},
	'd': {'s', 'f', 'e', 'r', 'x', 'c', 'v'}, 'f': {'d', 'g', 'r', 't', 'c', 'v', 'b'}, 'g': {'f', 'h', 't', 'y', 'v', 'b', 'n'},
	'h': {'g', 'j', 'y', 'u', 'b', 'n', 'm'}, 'j': {'h', 'k', 'u', 'i', 'n', 'm', ','}, 'k': {'j', 'l', 'i', 'o', 'm', ',', '.'},
	'l': {'k', ';', 'o', 'p', ',', '.', '/'}, 'z': {'a', 's', 'x'}, 'x': {'z', 'c', 's', 'd'}, 'c': {'x', 'v', 'd', 'f'},
	'v': {'c', 'b', 'f', 'g'}, 'b': {'v', 'n', 'g', 'h'}, 'n': {'b', 'm', 'h', 'j'}, 'm': {'n', ',', 'j', 'k'},
	' ': {' ', ' ', 'c', 'v', 'b', 'n', 'm'},
}

// min returns the smaller of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// humanType simulates human-like typing of the given text.
func humanType(text string) {
	logMessage("humanType: Starting to type text of length ", len(text))
	defer func() {
		if r := recover(); r != nil {
			logMessage("PANIC in humanType while typing: ", r, ". Text (first 50 chars): ", text[:min(50, len(text))])
		}
	}()

	for _, char := range text {
		if rand.Intn(100) < 2 {
			if near, ok := nearbyKeys[unicode.ToLower(char)]; ok && len(near) > 0 {
				wrongChar := near[rand.Intn(len(near))]
				robotgo.KeyTap(string(wrongChar))
				time.Sleep(time.Duration(rand.Intn(40)+60) * time.Millisecond)
				robotgo.KeyTap("backspace")
				time.Sleep(time.Duration(rand.Intn(20)+40) * time.Millisecond)
			}
		}
		robotgo.KeyTap(string(char))
		if rand.Intn(200) < 1 && char != ' ' && char != '\n' {
			time.Sleep(time.Duration(rand.Intn(80)+70) * time.Millisecond)
			robotgo.KeyTap("backspace")
			time.Sleep(time.Duration(rand.Intn(40)+50) * time.Millisecond)
			robotgo.KeyTap(string(char))
		}
		delay := rand.Intn(90) + 30
		if char == ' ' {
			delay += rand.Intn(70)
		} else if char == '\n' {
			delay += rand.Intn(130) + 70
		}
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	if rand.Intn(100) < 20 {
		time.Sleep(time.Duration(rand.Intn(200)+100) * time.Millisecond)
	}
}

// sanitizeName creates a valid Go identifier from a keyword and prefix.
func sanitizeName(keyword string, prefix string) string {
	if keyword == "" {
		keyword = "defaultName"
	}
	sanitizedKeywordPart := ""
	if len(keyword) > 0 && unicode.IsLetter(rune(keyword[0])) {
		sanitizedKeywordPart = string(keyword[0]) + keyword[1:]
	} else if len(keyword) > 0 {
		sanitizedKeywordPart = "G" + keyword
	} else {
		sanitizedKeywordPart = "G" + keyword
	}

	var sb strings.Builder
	firstChar := true
	for _, r := range sanitizedKeywordPart {
		if firstChar {
			if unicode.IsLetter(r) {
				sb.WriteRune(r)
				firstChar = false
			}
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
		}
	}
	resultKeyword := sb.String()
	if resultKeyword == "" {
		resultKeyword = "FallbackName"
	}
	finalName := prefix + strings.Title(resultKeyword)
	if prefix == "" {
		if finalName == "" {
			return "DefaultIdentifier"
		}
		if !unicode.IsLetter(rune(finalName[0])) {
			return "X" + finalName
		}
	}
	if finalName == prefix {
		return prefix + "Default"
	}
	return finalName
}

// sanitizeTypeName creates a valid Go type name from a keyword.
// It handles basic types, ensures custom types are capitalized, and removes invalid characters.
// This version is corrected to prevent stack overflow from recursion.
func sanitizeTypeName(keyword string) string {
	lowerKeyword := strings.ToLower(keyword)
	// Define basic types and their direct return values
	// This map helps in directly returning the correct form for basic Go types.
	basicTypesDirect := map[string]string{
		"string": "string", "int": "int", "bool": "bool", "float64": "float64", "byte": "byte", "rune": "rune", "error": "error",
		"any": "any", "uint": "uint", "int8": "int8", "int16": "int16", "int32": "int32", "int64": "int64",
		"uint8": "uint8", "uint16": "uint16", "uint32": "uint32", "uint64": "uint64", "uintptr": "uintptr", "float32": "float32",
	}

	// If the lowercased keyword is a basic type, return its predefined form directly.
	if directReturn, ok := basicTypesDirect[lowerKeyword]; ok {
		return directReturn
	}

	if keyword == "" {
		return "String" // Default to a common capitalized type if the keyword is empty.
	}

	r := []rune(keyword)
	// If the first character is not a letter, prepend "My" and TitleCase the rest.
	// This aims to create a valid identifier for custom types.
	if !unicode.IsLetter(r[0]) {
		var tempSb strings.Builder
		for _, char := range keyword { // Iterate through original keyword to build a clean part
			if unicode.IsLetter(char) || unicode.IsDigit(char) {
				tempSb.WriteRune(char)
			}
		}
		cleanPart := tempSb.String()
		if cleanPart == "" {
			return "MyDefaultType"
		} // Fallback if keyword had no letters/digits
		return "My" + strings.Title(cleanPart)
	}

	// Ensure the first letter is uppercase for custom types that don't start with non-letters.
	r[0] = unicode.ToUpper(r[0])

	// Build the cleaned name by keeping only letters and digits from the (now capitalized) keyword.
	var sb strings.Builder
	sb.WriteRune(r[0]) // Add the already capitalized first character.
	for i := 1; i < len(r); i++ {
		if unicode.IsLetter(r[i]) || unicode.IsDigit(r[i]) {
			sb.WriteRune(r[i])
		}
	}

	cleanName := sb.String()
	if len(cleanName) == 0 {
		// This should ideally not be reached if the keyword was not empty and had a first valid char.
		return "DefaultType"
	}

	// After forming cleanName (e.g., "MyCustomType", or "Int" from "iNt"):
	// Check if this cleanName, when lowercased, corresponds to a basic Go type.
	// This is to handle cases where the input keyword was a case-variant of a basic type
	// (e.g., "INT", "sTrInG") and wasn't caught by the initial `basicTypesDirect` lookup
	// because `lowerKeyword` was already handled.
	// If so, prepend "Custom" to distinguish it from the primitive type.
	lcCleanName := strings.ToLower(cleanName)
	if _, isBasic := basicTypesDirect[lcCleanName]; isBasic {
		// If lcCleanName (e.g., "int") is a key in basicTypesDirect,
		// it means cleanName (e.g., "Int") is a capitalized version of a basic type.
		// Since the initial check `basicTypesDirect[lowerKeyword]` would have caught
		// exact lowercase basic types (like "int" from keyword "int"),
		// reaching here implies the original keyword was likely a case-variant (e.g. "INT", "iNt")
		// that resulted in `cleanName` being "Int".
		// To avoid ambiguity, we prepend "Custom".
		return "Custom" + cleanName // e.g., "CustomInt", "CustomString"
	}

	return cleanName // Return the processed custom type name.
}

// generateRandomGoCode generates a random snippet of Go code.
func generateRandomGoCode() string {
	defer func() {
		if r := recover(); r != nil {
			logMessage("PANIC in generateRandomGoCode:", r)
		}
	}()

	var codeSnippet jen.Code
	elementType := rand.Intn(4)

	switch elementType {
	case 0: // Function
		funcName := sanitizeName(goKeywords[rand.Intn(len(goKeywords))], "handle")
		var statementsInFunc []jen.Code
		numStatementsInFunc := rand.Intn(3) + 1
		for i := 0; i < numStatementsInFunc; i++ {
			structureType := rand.Intn(4)
			switch structureType {
			case 0:
				varName := sanitizeName(goKeywords[rand.Intn(len(goKeywords))], "temp")
				typeName := sanitizeTypeName(goKeywords[rand.Intn(len(goKeywords))])
				if rand.Intn(2) == 0 {
					statementsInFunc = append(statementsInFunc, jen.Var().Id(varName).Id(typeName))
				} else {
					var val jen.Code
					switch typeName {
					case "string":
						val = jen.Lit("sample_text_" + goKeywords[rand.Intn(len(goKeywords))])
					case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "byte", "rune":
						val = jen.Lit(rand.Intn(1000))
					case "bool":
						val = jen.Lit(rand.Intn(2) == 0)
					case "float32", "float64":
						val = jen.Lit(rand.Float64() * 100.0)
					case "error":
						val = jen.Qual("errors", "New").Call(jen.Lit("local error example"))
					default:
						val = jen.Nil()
					}
					statementsInFunc = append(statementsInFunc, jen.Id(varName).Op(":=").Add(val))
				}
			case 1:
				condVar := sanitizeName(goKeywords[rand.Intn(len(goKeywords))], "is")
				statementsInFunc = append(statementsInFunc, jen.Id(condVar).Op(":=").Lit(rand.Intn(2) == 0))
				statementsInFunc = append(statementsInFunc,
					jen.If(jen.Id(condVar)).Block(
						jen.Qual("fmt", "Println").Call(jen.Lit("Condition true for "+condVar)),
					).Else().Block(
						jen.Qual("fmt", "Println").Call(jen.Lit("Condition false for "+condVar)),
					),
				)
			case 2:
				counterNameBase := goKeywords[rand.Intn(len(goKeywords))]
				var counter string
				if len(counterNameBase) > 0 {
					counter = sanitizeName(counterNameBase[0:min(1, len(counterNameBase))], "idx")
				} else {
					counter = "i" // Fallback if keyword was empty
				}
				if counter == "idx" && len(counterNameBase) > 1 { // try to make it more unique
					counter = sanitizeName(counterNameBase[0:min(2, len(counterNameBase))], "idx")
				}

				limit := rand.Intn(3) + 1
				statementsInFunc = append(statementsInFunc,
					jen.For(jen.Id(counter).Op(":=").Lit(0), jen.Id(counter).Op("<").Lit(limit), jen.Id(counter).Op("++")).Block(
						jen.Qual("fmt", "Println").Call(jen.Lit("Loop iteration:"), jen.Id(counter)),
					),
				)
			default:
				printArgs := []jen.Code{jen.Lit("Debug:")}
				numKeywordsToPrint := rand.Intn(2) + 1
				for k := 0; k < numKeywordsToPrint; k++ {
					printArgs = append(printArgs, jen.Lit(goKeywords[rand.Intn(len(goKeywords))]))
				}
				statementsInFunc = append(statementsInFunc, jen.Qual("fmt", "Println").Call(printArgs...))
			}
		}
		if len(statementsInFunc) == 0 {
			statementsInFunc = append(statementsInFunc, jen.Qual("fmt", "Println").Call(jen.Lit("Function "+funcName+" called")))
		}
		codeSnippet = jen.Func().Id(funcName).Params().Block(statementsInFunc...)

	case 1: // Struct
		baseName := goKeywords[rand.Intn(len(goKeywords))]
		structName := sanitizeTypeName(baseName)
		if unicode.IsLower([]rune(structName)[0]) || (sanitizeTypeName(structName) == structName && strings.Contains(" string int bool float64 error any uint int8 int16 int32 int64 uint8 uint16 uint32 uint64 uintptr float32 byte rune ", " "+structName+" ")) {
			structName = "App" + strings.Title(baseName) + "Model"
		}
		if len(structName) < 4 { // Ensure a reasonable length for type names
			structName = structName + "Struct"
		}

		var fields []jen.Code
		numFields := rand.Intn(3) + 1
		for i := 0; i < numFields; i++ {
			fieldNameSource := goKeywords[rand.Intn(len(goKeywords))]
			var fieldName string
			if len(fieldNameSource) > 0 {
				// Sanitize field name, ensure it's TitleCased for export. Empty prefix for general identifier.
				rawFieldName := sanitizeName(fieldNameSource, "")
				if len(rawFieldName) > 0 {
					fieldName = strings.Title(string(unicode.ToLower(rune(rawFieldName[0])))) + rawFieldName[1:] // Ensure first letter is upper, rest follows. More robust Title.
					fieldName = strings.Title(rawFieldName)                                                      // Simpler Title casing often works well for Go style.
				} else {
					fieldName = "Field" + fmt.Sprint(i+1)
				}
			} else {
				fieldName = "Property" + fmt.Sprint(i+1) // Fallback field name
			}
			if !unicode.IsUpper(rune(fieldName[0])) && len(fieldName) > 0 { // Double check first letter is Upper for exported field
				fieldName = strings.Title(fieldName)
			}

			fieldType := sanitizeTypeName(goKeywords[rand.Intn(len(goKeywords))])
			fields = append(fields, jen.Id(fieldName).Id(fieldType))
		}
		if len(fields) == 0 {
			fields = append(fields, jen.Comment("Struct with no fields definition"))
		}
		codeSnippet = jen.Type().Id(structName).Struct(fields...)

	case 2: // Var declaration (top-level)
		varName := sanitizeName(goKeywords[rand.Intn(len(goKeywords))], "global")
		typeName := sanitizeTypeName(goKeywords[rand.Intn(len(goKeywords))])

		if rand.Intn(3) == 0 {
			codeSnippet = jen.Var().Id(varName).Id(typeName)
		} else {
			var val jen.Code
			switch typeName {
			case "string":
				val = jen.Lit("global_string_val_" + goKeywords[rand.Intn(len(goKeywords))])
			case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "byte", "rune":
				val = jen.Lit(rand.Intn(10000))
			case "bool":
				val = jen.Lit(rand.Intn(2) == 0)
			case "float32", "float64":
				val = jen.Lit(rand.Float64() * 1000.0)
			case "error":
				val = jen.Qual("errors", "New").Call(jen.Lit("global default error state"))
			default:
				if strings.HasSuffix(typeName, "Struct") || strings.HasSuffix(typeName, "Data") || strings.HasSuffix(typeName, "Model") || strings.HasSuffix(typeName, "Config") {
					val = jen.Id(typeName).Values()
				} else {
					val = jen.Nil()
				}
			}
			codeSnippet = jen.Var().Id(varName).Id(typeName).Op("=").Add(val)
		}

	case 3: // Const declaration (top-level)
		constNameSource := goKeywords[rand.Intn(len(goKeywords))]
		var constName string
		if len(constNameSource) > 0 {
			constName = strings.Title(sanitizeName(constNameSource, "Max"))
			if len(constName) > 0 && !unicode.IsLetter(rune(constName[0])) {
				constName = "C" + constName
			} else if constName == "" {
				constName = "DefaultConstantName"
			}
		} else {
			constName = "DefaultConst"
		}
		if constName == "Max" {
			constName = "MaxDefaultValue"
		} // Avoid just "Max"

		constTypeChoice := rand.Intn(3)
		var constValue jen.Code
		var constTypeName jen.Code

		switch constTypeChoice {
		case 0:
			constValue = jen.Lit(rand.Intn(5000) + 1)
			constTypeName = jen.Id("int")
		case 1:
			constValue = jen.Lit("config_key_" + goKeywords[rand.Intn(len(goKeywords))])
			constTypeName = jen.Id("string")
		default:
			constValue = jen.Lit(rand.Intn(2) == 0)
			constTypeName = jen.Id("bool")
		}

		if rand.Intn(2) == 0 {
			codeSnippet = jen.Const().Id(constName).Add(constTypeName).Op("=").Add(constValue)
		} else {
			codeSnippet = jen.Const().Id(constName).Op("=").Add(constValue)
		}
	}

	if codeSnippet == nil {
		logMessage("generateRandomGoCode: codeSnippet was nil, generating fallback comment.")
		codeSnippet = jen.Comment("Fallback: unable to generate specific code snippet.")
	}

	var sb strings.Builder
	tempFile := jen.NewFile("temp_package") // Use a temporary package name
	tempFile.Add(codeSnippet)

	if err := tempFile.Render(&sb); err != nil {
		logMessage("ERROR rendering generated code snippet:", err)
		return fmt.Sprintf("// Error rendering snippet: %v\nfunc renderErrorRecovery(){}\n\n", err)
	}

	generatedStr := sb.String()
	packageHeader := "package temp_package\n\n"
	if strings.HasPrefix(generatedStr, packageHeader) {
		generatedStr = strings.TrimPrefix(generatedStr, packageHeader)
	}

	generatedStr += "\n\n"

	if strings.TrimSpace(generatedStr) == "" {
		logMessage("Generated empty snippet, using placeholder.")
		return "// activity placeholder: empty snippet generated\nfunc placeholderForEmptySnippet() {}\n\n"
	}
	return generatedStr
}

// preventComputerSleep periodically moves the mouse and presses a key to prevent sleep/screensaver.
func preventComputerSleep() {
	logMessage("preventComputerSleep goroutine started.")
	defer func() {
		if r := recover(); r != nil {
			logMessage("PANIC in preventComputerSleep:", r)
		}
		logMessage("preventComputerSleep goroutine stopped.")
	}()

	for {
		minSleep := 20 * time.Second
		maxSleep := 40 * time.Second
		sleepDuration := minSleep + time.Duration(rand.Int63n(int64(maxSleep-minSleep)))
		time.Sleep(sleepDuration)

		dx := rand.Intn(15) + 10
		dy := rand.Intn(15) + 10
		if rand.Intn(2) == 0 {
			dx = -dx
		}
		if rand.Intn(2) == 0 {
			dy = -dy
		}

		currentX, currentY := robotgo.Location()
		robotgo.MoveRelative(dx, dy)
		time.Sleep(time.Duration(rand.Intn(100)+50) * time.Millisecond)

		robotgo.KeyTap("shift")
		time.Sleep(time.Duration(rand.Intn(100)+50) * time.Millisecond)

		finalX, finalY := robotgo.Location()
		robotgo.MoveRelative(currentX-finalX, currentY-finalY)
	}
}

// monitorMouseExitCondition checks if the mouse cursor enters a defined top-left screen area.
func monitorMouseExitCondition(sigs chan<- os.Signal, thresholdX, thresholdY int) {
	logMessage("monitorMouseExitCondition goroutine started. Exit zone: <", thresholdX, ",<", thresholdY)
	defer func() {
		if r := recover(); r != nil {
			logMessage("PANIC in monitorMouseExitCondition:", r)
		}
		logMessage("monitorMouseExitCondition goroutine stopped.")
	}()

	for {
		x, y := robotgo.Location()
		if x < thresholdX && y < thresholdY && x >= 0 && y >= 0 {
			logMessage("monitorMouseExitCondition: Mouse in EXIT ZONE (", x, ",", y, "). Signaling termination.")
			fmt.Printf("\nMouse entered exit zone (%d, %d). Terminating...\n", x, y)
			sigs <- syscall.SIGTERM
			return
		}
		time.Sleep(1500 * time.Millisecond)
	}
}

// generateCodeInBursts manages the cycle of active coding bursts and pauses.
func generateCodeInBursts(maxIntervalBetweenBursts, maxBurstDuration, intervalBetweenTyping time.Duration) {
	logMessage("generateCodeInBursts goroutine started.")
	iterationCount := 0
	defer func() {
		if r := recover(); r != nil {
			logMessage("PANIC in generateCodeInBursts at iteration", iterationCount, ":", r)
		}
		logMessage("generateCodeInBursts goroutine stopped.")
	}()

	for {
		iterationCount++
		logMessage("generateCodeInBursts: Starting burst cycle #", iterationCount)

		burstDuration := time.Duration(rand.Int63n(int64(maxBurstDuration))) + 30*time.Second
		if maxBurstDuration < 30*time.Second {
			burstDuration = maxBurstDuration
		}
		if burstDuration <= 0 {
			burstDuration = 30 * time.Second
		}
		logMessage("generateCodeInBursts: Active coding burst for approximately ", burstDuration)
		fmt.Printf("Starting coding burst for about %s...\n", burstDuration.Round(time.Second))
		endTime := time.Now().Add(burstDuration)

		burstCodeBlockCount := 0
		for time.Now().Before(endTime) {
			burstCodeBlockCount++
			codeToType := generateRandomGoCode()
			humanType(codeToType)

			interCodePauseBase := intervalBetweenTyping
			if interCodePauseBase < 500*time.Millisecond {
				interCodePauseBase = 500 * time.Millisecond
			}
			interCodePause := time.Duration(rand.Int63n(int64(interCodePauseBase))) + (interCodePauseBase / 2)
			if interCodePause <= 0 {
				interCodePause = 500 * time.Millisecond
			}

			fmt.Printf("Brief pause for %s...\n", interCodePause.Round(time.Second))
			time.Sleep(interCodePause)

			if time.Now().After(endTime) {
				logMessage("generateCodeInBursts: Burst time ended during inter-code pause.")
				break
			}
		}
		logMessage("generateCodeInBursts: Burst cycle #", iterationCount, " ended. Typed ", burstCodeBlockCount, " code blocks.")
		fmt.Printf("Coding burst #%d finished. Typed %d code blocks.\n", iterationCount, burstCodeBlockCount)

		pauseDurationBase := maxIntervalBetweenBursts
		if pauseDurationBase < 1*time.Minute {
			pauseDurationBase = 1 * time.Minute
		}
		pauseDuration := time.Duration(rand.Int63n(int64(pauseDurationBase))) + 30*time.Second
		if pauseDuration <= 0 {
			pauseDuration = time.Minute
		}

		logMessage("generateCodeInBursts: Pausing between bursts for ", pauseDuration)
		fmt.Printf("Taking a break for about %s before next coding burst...\n", pauseDuration.Round(time.Second))
		time.Sleep(pauseDuration)
	}
}

func main() {
	internal.HandleStartup()
	initLogger()
	logMessage("-------------------- Program Starting --------------------")
	defer func() {
		if r := recover(); r != nil {
			logMessage("PANIC in main:", r)
			fmt.Println("A critical error occurred. Check console log for details.")
		}
		logMessage("-------------------- Program Exiting --------------------")
	}()

	intervalRange := flag.Duration("interval-range", 8*time.Minute, "Maximum PAUSE duration between typing bursts (e.g., 30m, 1h)")
	burstRange := flag.Duration("burst-range", 7*time.Minute, "Maximum active typing burst duration (e.g., 5m, 15m)")
	intervalBetweenTyping := flag.Duration("interval-between-typing", 7*time.Second, "Base interval between typing new code blocks within a burst (e.g., 5s, 10s)")
	exitCoordinateX := flag.Int("exit-x", 50, "X-coordinate threshold for mouse exit zone (top-left corner)")
	exitCoordinateY := flag.Int("exit-y", 50, "Y-coordinate threshold for mouse exit zone (top-left corner)")
	flag.Parse()

	logMessage("Flags: interval-range=", *intervalRange, ", burst-range=", *burstRange,
		", interval-between-typing=", *intervalBetweenTyping, ", exit-x=", *exitCoordinateX, ", exit-y=", *exitCoordinateY)

	fmt.Printf("Configuration: Max pause between bursts: %s, Max burst duration: %s, Interval in burst: %s\n", *intervalRange, *burstRange, *intervalBetweenTyping)
	fmt.Printf("To exit: Press Ctrl+C, or move mouse to screen coordinates x < %d and y < %d.\n", *exitCoordinateX, *exitCoordinateY)
	fmt.Println("Starting simulation in 3 seconds...")
	time.Sleep(3 * time.Second)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	go preventComputerSleep()
	go generateCodeInBursts(*intervalRange, *burstRange, *intervalBetweenTyping)
	go monitorMouseExitCondition(sigs, *exitCoordinateX, *exitCoordinateY)

	receivedSignal := <-sigs
	logMessage("Termination signal received: ", receivedSignal.String())
	fmt.Println("\nTermination signal (", receivedSignal.String(), ") received. Exiting program gracefully.")
}
