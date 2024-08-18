# Example of DevContainers Usage

This repository demonstrates how [devcontainers](https://containers.dev/) can be used to streamline development workflows, particularly in a microservice architecture where multiple apps and languages are used.

## Content
For this repo I implemented two basic `good citizen` apps: one written in Python and the other in Golang. Both applications feature a similar HTTP interface, utilize PostgreSQL as their backend, and include tests and Makefiles, representing typical microservices. For both apps I also added `devcontainers` config. So, both apps can be run on any machine with all dependencies within seconds. 

## TODO
* (done) Python App
* (done) Golang App
* (done) Basic readme
* (WIP) Blog post to explain step-by-step how devcontainers were configuired for these apps 
* Show how devcontainers can be used in CI/CD
* Example how lambda can be used inside devcontainers with AWS SAM


## Prerequisites
*  Docker must be installed on your machine, or you should have access to a remote Docker environment.
* _No need_ to manually install any application dependencies—they'll be handled within the DevContainer.

 
## Usage
You can seamlessly use this setup in the console (e.g., if you use VIM/NVIM) with the DevContainers CLI or within an IDE (tested with VSCode, GoLand, and PyCharm). Please note that by default, you cannot run both the Python and Golang apps simultaneously, as they both bind to port 8000. However, you can easily modify this by assigning different ports to each application.

### Using DevContainers CLI
To test DevContainers with any of the apps using the [devcontainers-cli](https://github.com/devcontainers/cli), navigate to the desired app folder and run:
```
devcontainer --workspace-folder=. up
devcontainer exec --workspace-folder=. make
```
From here, you can modify the app's code and test it by sending HTTP requests to localhost on port 8000.

### Using Visual Studio Code (VSCode)
* Open either the `python_app` or `go_app` folder as a project in VSCode.
* VSCode will prompt you to open the project in a DevContainer.
* In the integrated terminal, run `make`

Now you can develop the app in VSCode as usual. All dependencies are isolated within the DevContainer.

### Using GoLand/PyCharm
* Open either the `python_app` or `go_app` folder in GoLand/PyCharm.
* Right-click on the devcontainer.json file inside the .devcontainer directory and select `Create Dev Container and Mount Source`.
* A new IDE window will open, running inside the DevContainer with all dependencies installed.

You can start the app by running make in the IDE terminal and continue development as you would locally.

## Testing 
Once the DevContainer is running, the Python or Golang app will be accessible on port 8000 on your localhost. Python app uses [FastAPI](https://fastapi.tiangolo.com/), so you can check API spec by opening the browser `http://127.0.0.1:8000/docs`

To use app itself, you can execute the following commands:
* Create item:
```
curl -XPOST -H 'Content-Type: application/json' http://127.0.0.1:8000 -d '{"item_id": "111", "value": "222"}'
```
* Fetch Item:
```
curl -H 'Content-Type: application/json' http://127.0.0.1:8000/111
```

