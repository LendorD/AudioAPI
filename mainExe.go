package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: myexe.exe arg1 arg2")
		os.Exit(1)
	}

	arg1 := os.Args[1]
	arg2 := os.Args[2]

	fmt.Println("Started test process...")
	fmt.Printf("Arguments received: %s, %s\n", arg1, arg2)

	// Имитируем "долгую работу"
	time.Sleep(10 * time.Second)

	// Пишем в файл
	f, err := os.Create("output.txt")
	if err != nil {
		fmt.Println("Error creating output.txt:", err)
		os.Exit(1)
	}
	defer f.Close()

	_, err = f.WriteString(fmt.Sprintf("Processed arguments: %s | %s\n", arg1, arg2))
	if err != nil {
		fmt.Println("Error writing to output.txt:", err)
		os.Exit(1)
	}

	fmt.Println("Process finished successfully.")
}
