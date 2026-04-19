package main

import (
	"fmt"
	"gomux/pkg/gomux"
)

func main() {
	fmt.Println("Testing Alacritty FFI...")
	
	// Create terminal
	term, err := gomux.NewAlacrittyTerm(10, 40)
	if err != nil {
		fmt.Printf("Error creating terminal: %v\n", err)
		return
	}
	defer term.Close()
	
	// Test 1: Basic text
	fmt.Println("\nTest 1: Basic text input")
	term.ProcessBytes([]byte("Hello World"))
	fmt.Println(term.Render(2))
	
	// Test 2: Newlines
	fmt.Println("\nTest 2: Newlines")
	term.ProcessBytes([]byte("\r\nSecond line"))
	fmt.Println(term.Render(3))
	
	// Test 3: Backspace
	fmt.Println("\nTest 3: Backspace")
	term.ProcessBytes([]byte("\r\nTest"))
	term.ProcessBytes([]byte{0x08}) // BS
	fmt.Println(term.Render(4))
	
	// Test 4: Clear screen
	fmt.Println("\nTest 4: Clear screen (ESC[2J)")
	term.ProcessBytes([]byte{0x1b, '[', '2', 'J'})
	fmt.Println(term.Render(5))
	fmt.Println("(should be empty after clear)")
	
	// Test 5: Cursor position
	fmt.Println("\nTest 5: Cursor positioning")
	term.ProcessBytes([]byte{0x1b, '[', '5', ';', '1', '0', 'H'}) // ESC[5;10H
	term.ProcessBytes([]byte("X"))
	row, col := term.GetCursor()
	fmt.Printf("Cursor at: row=%d, col=%d\n", row, col)
	fmt.Println(term.Render(6))
	
	fmt.Println("\nAll tests completed!")
}
