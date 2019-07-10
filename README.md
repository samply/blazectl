# blazectl

blazectl is a command line tool to control your FHIR® server. blazectl also works with [Blaze][4].

Currently you can upload transaction bundles from a directory and count resources.

## Installation

blazectl is written in Go. All you need is a single binary which is available for Linux, macOS and Windows.

### Linux

1. Download the latest release with the command:

   ```bash
   curl -LO https://github.com/life-research/blazectl/releases/download/v0.1.0/blazectl-0.1.0-linux-amd64.tar.gz
   ```

1. Untar the binary:

   ```bash
   tar xzf blazectl-0.1.0-linux-amd64.tar.gz
   ```
   
1. Move the binary in to your PATH.

   ```bash
   sudo mv ./blazectl /usr/local/bin/blazectl
   ```

1. Test to ensure the version you installed is up-to-date:

   ```bash
   blazectl --version
   ```

### macOS

1. Download the latest release with the command:

   ```bash
   curl -LO https://github.com/life-research/blazectl/releases/download/v0.1.0/blazectl-0.1.0-darwin-amd64.tar.gz
   ```

1. Untar the binary:

   ```bash
   tar xzf blazectl-0.1.0-darwin-amd64.tar.gz
   ```
   
1. Move the binary in to your PATH.

   ```bash
   sudo mv ./blazectl /usr/local/bin/blazectl
   ```

1. Test to ensure the version you installed is up-to-date:

   ```bash
   blazectl --version
   ```

### Windows

1. Download the latest release [here][3]

1. Unzip the binary.

1. Add the binary in to your PATH.

1. Test to ensure the version you downloaded is up-to-date:

   ```
   blazectl --version
   ```
   
## Usage

```
$ blazectl
Usage:
  blazectl [command]

Available Commands:
  count-resources Counts all resources by type
  help            Help about any command
  upload          Upload transaction bundles

Flags:
  -h, --help            help for blazectl
      --server string   the base URL of the server to use

Use "blazectl [command] --help" for more information about a command.
```

## Similar Software

* [VonkLoader][1] - can also upload transaction bundles but needs .NET SDK
* [Synthea Uploader][2] - no parallel uploads

## License

Copyright © 2019 Alexander Kiel <alexander.kiel@life.uni-leipzig.de>

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

[1]: <http://docs.simplifier.net/vonkloader/>
[2]: <https://github.com/synthetichealth/uploader>
[3]: <https://github.com/life-research/blazectl/releases/download/v0.1.0/blazectl-0.1.0-windows-amd64.zip>
[4]: <https://github.com/life-research/blaze>
