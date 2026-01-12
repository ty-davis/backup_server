package main

import (
	"backup_server/internal/database"
	"log"
	"os"
)

func addFile(
	db *database.DB,
	fileName string,
	filePath string,
	groupID int,
	description string,
) {

	if _, err := os.Stat(filePath); err == nil {
		log.Printf("Adding file: %s (%s) \n", fileName, filePath)
		db.AddFile(fileName, filePath, groupID, description)
	} else {
		log.Printf("Skipping file: %s (%s) %v\n", fileName, filePath, err)
	}
}

func main() {
	db, err := database.InitDB("backup_server.db")
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	log.Println("Creating groups...")
	adminGroupID, _ := db.CreateGroup("admins")

	log.Println("Creating users...")
	db.CreateUser("admin", "admin", []int{int(adminGroupID)})

	log.Println("Database initialized successfully!")
	log.Println("Default users:")
	log.Println("  admin/admin (admins group)")
}
