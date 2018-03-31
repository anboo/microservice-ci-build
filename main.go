package main

import (
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"encoding/json"
	"github.com/satori/go.uuid"
)

type Error struct {
	Code         int    `json:"code"`
	ErrorMessage string `json:"error_message"`
}

func Serialize(entity interface{}) []byte {
	var res, err = json.Marshal(entity)
	if err != nil {
		return Serialize(Error{
			Code:         500,
			ErrorMessage: "Syntax error",
		})
	}

	return res
}

type Command struct {
	Cmd string
}

type BuildItem struct {
	Uuid   string  `json:"uuid"`
	Tasks []Command `json:"tasks"`
	ProjectId string `json:"project_id"`
	DockerImage string `json:"docker_image"`
	IpAddress string `json:"ip_address"`
	Done bool `json:"done"`
}

var builds []BuildItem

func listBuildsHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Write(Serialize(builds))
}

func listProjectBuildsHandler(writer http.ResponseWriter, request *http.Request)  {
	params := mux.Vars(request)

	prepareBuilds := []BuildItem{}
	for _, build := range(builds) {
		if build.ProjectId == params["uuid"] {
			prepareBuilds = append(prepareBuilds, build)
		}
	}

	writer.Write(Serialize(prepareBuilds))
}

func viewBuildHandler(writer http.ResponseWriter, request *http.Request) {
	var params = mux.Vars(request)

	for _, build := range builds {
		if build.Uuid == params["uuid"] {
			writer.Write(Serialize(build))
			return
		}
	}
}

func addBuildHandler(writer http.ResponseWriter, request *http.Request) {
	var build BuildItem
	_ = json.NewDecoder(request.Body).Decode(&build)

	build.Uuid = uuid.Must(uuid.NewV4()).String();

	go startBuildProcess(&build)

	builds = append(builds, build)

	writer.Write(Serialize(build))
}

func startBuildProcess(item *BuildItem)  {
	imageName := item.DockerImage

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(); if err != nil { panic(err) }

	out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{}); if err != nil { panic(err) }
	defer out.Close();
	//io.Copy(os.Stdout, out)

	resp, err := cli.ContainerCreate(ctx, &container.Config{ Image: imageName }, nil, nil, ""); if err != nil { panic(err) }
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil { panic(err) }

	insp, _ := cli.ContainerInspect(ctx, resp.ID)
	item.IpAddress = insp.NetworkSettings.IPAddress

	tmpBuilds := []BuildItem{}
	for _, build := range builds {
		if build.Uuid == item.Uuid {
			build.IpAddress = insp.NetworkSettings.IPAddress
		}
		tmpBuilds = append(tmpBuilds, build)
	}
	builds = tmpBuilds

	defer cli.Close();
	//cli.ContainerKill(ctx, resp.ID, "9");
}

func main() {
	router := mux.NewRouter()

	router.HandleFunc("/builds", listBuildsHandler).Methods("GET")
	router.HandleFunc("/build/{uuid}", viewBuildHandler).Methods("GET")
	router.HandleFunc("/build/project/{uuid}", listProjectBuildsHandler).Methods("GET")
	router.HandleFunc("/builds", addBuildHandler).Methods("POST")

	log.Fatal(http.ListenAndServe(":9000", router))
}