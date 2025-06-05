package main

import (
	"log"
	"os"

	"FP-DevOps/config"
	"FP-DevOps/controller"
	"FP-DevOps/middleware"
	"FP-DevOps/migrations/seeder"
	"FP-DevOps/repository"
	"FP-DevOps/routes"
	"FP-DevOps/service"

	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
	"gorm.io/gorm"
)

func main() {
	var (
		db         *gorm.DB          = config.SetUpDatabaseConnection()
		jwtService config.JWTService = config.NewJWTService()

		userRepository repository.UserRepository = repository.NewUserRepository(db)
		fileRepository repository.FileRepository = repository.NewFileRepository(db)

		userService service.UserService = service.NewUserService(userRepository)
		fileService service.FileService = service.NewFileService(fileRepository)

		userController controller.UserController = controller.NewUserController(userService, jwtService)
		fileController controller.FileController = controller.NewFileController(fileService, jwtService)
		viewController controller.ViewController = controller.NewViewController(jwtService)
	)

	server := gin.Default()
	server.Use(middleware.CORSMiddleware())
	server.LoadHTMLGlob("templates/*")

	routes.User(server, userController, jwtService)
	routes.File(server, fileController, jwtService)
	routes.View(server, viewController, jwtService)

	if err := seeder.RunSeeders(db); err != nil {
		log.Fatalf("error migration seeder: %v", err)
		return
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8888"
	}

	if err := server.Run(":" + port); err != nil {
		log.Fatalf("error running server: %v", err)
	}
}
