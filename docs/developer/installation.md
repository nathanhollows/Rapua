---
title: "Installation"
sidebar: true
order: 1
---

# Installation

These instructions will get you a copy of the project up and running on your local machine for development and testing purposes.

## Built With

Rapua is built with the GOTH stack: Go, (SQLite), HTMX, and TailwindCSS.

[![][go]][go-url] [![][htmx]][htmx-url]

## Prerequisites

Ensure you have Go installed on your machine. If not, you can download it from the official website [here](https://golang.org/). Make sure the version is at least what is shown in the badge above. You can check the version by running the following command in your terminal:

```sh
go version
```

You will also need to have SQLite installed on your machine. If you don't have it installed, you can download it from the official website [here](https://www.sqlite.org/download.html).

## Installing

1. Clone the repo
   ```sh
   git clone https://github.com/nathanhollows/Rapua.git
   ```
2. Change into the project directory
   ```sh
    cd Rapua
    ```
3. Set the .env file
    ```sh
    cp .env.template .env
    ```
    Update the .env file with your database details
    ```sh
    vi .env
    ```
4. Build the project
    ```sh
    make build
    ```
    Other build options are available including `make dev`, `make tailwind-build`, `make tailwind-watch`, `make templ-watch`, `make templ-generate`, and `make test`.
5. Run [database migrations](/docs/developer/migrations)
    ```sh
    ./rapua db migrate
    ```
6. Set folder permissions (Unix only)
    ```sh
    mkdir -p ./static/uploads
    chown -R $(whoami) ./static/uploads
    ```
    This is required for the application to be able to write files to the uploads directory. You can change this to a more secure setting later.
7. Run the project
    ```sh
    ./rapua
    ```
8. Open your browser and navigate to `http://localhost:8090`
    

<p align="right">(<a href="#readme-top">back to top</a>)</p>



<!-- MARKDOWN LINKS & IMAGES -->
<!-- https://www.markdownguide.org/basic-syntax/#reference-style-links -->
[contributors-shield]: https://img.shields.io/github/contributors/nathanhollows/Rapua.svg?style=for-the-badge
[contributors-url]: https://github.com/nathanhollows/Rapua/graphs/contributors
[forks-shield]: https://img.shields.io/github/forks/nathanhollows/Rapua.svg?style=for-the-badge
[forks-url]: https://github.com/nathanhollows/Rapua/network/members
[stars-shield]: https://img.shields.io/github/stars/nathanhollows/Rapua.svg?style=for-the-badge
[stars-url]: https://github.com/nathanhollows/Rapua/stargazers
[issues-shield]: https://img.shields.io/github/issues/nathanhollows/rapua.svg?style=for-the-badge
[issues-url]: https://github.com/nathanhollows/Rapua/issues
[license-shield]: https://img.shields.io/github/license/nathanhollows/Rapua.svg?style=for-the-badge
[license-url]: https://github.com/nathanhollows/Rapua/blob/master/LICENSE
[product-screenshot]: images/screenshot.png
[go]: https://img.shields.io/github/go-mod/go-version/nathanhollows/Rapua?style=for-the-badge
[go-url]: https://go.dev/
[htmx]: https://img.shields.io/badge/HTMX-36C?logo=htmx&logoColor=fff&style=for-the-badge
[htmx-url]: https://htmx.org/
