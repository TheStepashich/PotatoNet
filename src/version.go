package backend

// Version — версия приложения. Единственный источник — файл VERSION в корне
// репозитория; makefile подставляет значение при сборке во все артефакты:
//
//	-ldflags "-X potatonet/src.Version=1.2.3"
var Version = "dev"
