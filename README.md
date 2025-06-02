<!-- Improved compatibility of back to top link: See: https://github.com/othneildrew/Best-README-Template/pull/73 -->
<a id="readme-top"></a>


<!-- PROJECT SHIELDS -->
[![Contributors][contributors-shield]][contributors-url]
[![Forks][forks-shield]][forks-url]
[![Stargazers][stars-shield]][stars-url]
[![Issues][issues-shield]][issues-url]
[![MIT License][license-shield]][license-url]
[![LinkedIn][linkedin-shield]][linkedin-url]



<!-- PROJECT LOGO -->
<br />
<div align="center">
  <!-- <a href="https://github.com/nathanhollows/Rapua"> -->
  <!--   <img src="images/logo.png" alt="Logo" width="80" height="80"> -->
  <!-- </a> -->

  <h3 align="center">Rapua</h3>

  <p align="center">
        Navigating learning, made easy.
    <br />
    <a href="https://rapua.nz">See in action</a>
    ·
    <a href="https://github.com/nathanhollows/Rapua/issues/new?assignees=&labels=&projects=&template=bug_report.md">Report Bug</a>
    ·
    <a href="https://github.com/nathanhollows/Rapua/issues/new?assignees=&labels=&projects=&template=feature_request.md">Request Feature</a>
    ·
    <a href="https://rapua.nz/docs/">Read the Docs</a>
  </p>
</div>



<!-- TABLE OF CONTENTS -->
<details>
  <summary>Table of Contents</summary>
  <ol>
    <li>
      <a href="#about-the-project">About The Project</a>
      <ul>
        <li><a href="#built-with">Built With</a></li>
      </ul>
    </li>
    <li>
      <a href="#getting-started">Getting Started</a>
      <ul>
        <li><a href="#prerequisites">Prerequisites</a></li>
        <li><a href="#installation">Installation</a></li>
      </ul>
    </li>
    <li><a href="#usage">Usage</a></li>
    <li><a href="#roadmap">Roadmap</a></li>
    <li><a href="#contributing">Contributing</a></li>
    <li><a href="#license">License</a></li>
    <li><a href="#contact">Contact</a></li>
    <li><a href="#acknowledgments">Acknowledgments</a></li>
  </ol>
</details>



<!-- ABOUT THE PROJECT -->
## About The Project

Rapua is an open-source game platform designed for place-based learning. Rapua is the culmination of two key projects: [*The Amazing Trace*](https://github.com/nathanhollows/AmazingTrace), developed as part of my [Master of Science Communication thesis](https://ourarchive.otago.ac.nz/esploro/outputs/9926546072901891) at the University of Otago, and Te Rapu Hamu, which I built for the Faculty of Law at the University of Otago to support their vision.

Rapua exists to make it easy to create games for education in the real world. It combines the best of both platforms, offering a powerful tool for learning that can teach complex concepts, engage diverse audiences, and create immersive, real-world educational experiences. It can be applied to a wide range of educational contexts, from university orientation and induction, staff training, and health and safety, to community engagement and public outreach.

<p align="right">(<a href="#readme-top">back to top</a>)</p>



### Built With

Rapua is built with the [GOTTH stack](https://github.com/TomDoesTech/GOTTH): Go, (SQLite), TailwindCSS, Templ, and HTMX.

[![Go][go]][go-url] [![HTMX][htmx]][htmx-url]

<p align="right">(<a href="#readme-top">back to top</a>)</p>



<!-- GETTING STARTED -->
## Getting Started

To get a local copy up and running follow these simple steps. If you would prefer a hosted version, you can find it [here](https://rapua.nz).

### Prerequisites

Ensure you have Go installed on your machine. If not, you can download it from the official website [here](https://golang.org/). Make sure the version is at least what is shown in the badge above. You can check the version by running the following command in your terminal:

```sh
go version
```

You will also need to have SQLite installed on your machine. If you don't have it installed, you can download it from the official website [here](https://www.sqlite.org/download.html).

### Installation

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
3. Build the project
    ```sh
    make build
    ```
    Other build options are available including `make dev`, `make tailwind-build`, `make tailwind-watch`, `make templ-watch`, `make templ-generate`, and `make test`.
4. Run the project
    ```sh
    ./rapua
    ```
5. Open your browser and navigate to `http://localhost:8090`
    

<p align="right">(<a href="#readme-top">back to top</a>)</p>



<!-- USAGE EXAMPLES -->
## Usage

For examples of how to use Rapua, please refer to the [Docs](https://rapua.nz/docs).

<p align="right">(<a href="#readme-top">back to top</a>)</p>



<!-- ROADMAP -->
## Roadmap

See the [roadmap/wishlist](https://rapua.nz/docs/developer/roadmap) for a list of proposed features. The list is not exhaustive and is subject to change. Please [request a feature](https://github.com/nathanhollows/Rapua/issues/new?assignees=&labels=&projects=&template=feature_request.md) if you would like to see something added.

<p align="right">(<a href="#readme-top">back to top</a>)</p>



<!-- CONTRIBUTING -->
## Contributing

Contributions are what make the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

If you have a suggestion that would make this better, please fork the repo and create a pull request. You can also simply open an issue with the tag "enhancement".
Don't forget to give the project a star! Thanks again!

1. Fork the Project
2. Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3. Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the Branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

<!-- ### Top contributors: -->
<!---->
<!-- <a href="https://github.com/nathanhollows/Rapua/graphs/contributors"> -->
<!--   <img src="https://contrib.rocks/image?repo=nathanhollows/Rapua" alt="contrib.rocks image" /> -->
<!-- </a> -->

<p align="right">(<a href="#readme-top">back to top</a>)</p>



<!-- LICENSE -->
## License

Distributed under the MIT License. See `LICENSE.txt` for more information.

<p align="right">(<a href="#readme-top">back to top</a>)</p>



<!-- CONTACT -->
## Contact

Nathan Hollows - nathan@rapua.nz 

[![LinkedIn][linkedin-shield]][linkedin-url]

<p align="right">(<a href="#readme-top">back to top</a>)</p>



<!-- ACKNOWLEDGMENTS -->
## Acknowledgements

* The University of Otago for supporting the research and development of this project and its predecessors.
    * The Department of Science Communication for their guidance and support.
    * The Faculty of Law for the opportunity to work with them on such an exciting project.
    * The Higher Education Development Centre for their support.
    * The Locals Collegiate Community, Pacific Islands Centre, the Sub-Warden training committee, and the College of Education for their support and feedback.


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
[linkedin-shield]: https://img.shields.io/badge/-LinkedIn-black.svg?style=for-the-badge&logo=linkedin&colorB=555
[linkedin-url]: https://linkedin.com/in/nathanhollows
[product-screenshot]: images/screenshot.png
[go]: https://img.shields.io/github/go-mod/go-version/nathanhollows/Rapua?style=for-the-badge
[go-url]: https://go.dev/
[htmx]: https://img.shields.io/badge/HTMX-36C?logo=htmx&logoColor=fff&style=for-the-badge
[htmx-url]: https://htmx.org/
