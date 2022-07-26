# hclconv

Simple tool to convert JSON files to HCL2 files and vice versa. It was intended for personal use and therefore its implementation might have a few flaws (I'm not a Go expert, but I have been using it for a while and I really like it).

## Installation

Download the binary fitting your platform from the [releases](https://github.com/rescDev/hclconv/releases) page.

## Usage

## Convert HCL file to JSON

```bash
hclconv --in sample.tfvars --out sample.tfvars.json
```

## Convert JSON file to HCL

```bash
hclconv --in sample.tfvars.json --out sample.tfvars
```

## Convert JSON file to HCL and format its content

**NOTE**: This requires the [`terraform`](https://github.com/hashicorp/terraform) CLI tool to be installed as the format is done by running `terraform fmt <output-file>`.

```bash
hclconv --format --in sample.tfvars.json --out sample.tfvars
```

## Contributing

Contributions are very welcome ! As written initially, there might be a few things that can be improved. So if you stumble across this project and want to add things or improve the implementation, feel free to propose your changes :)

## Maintainers

Rene Schach - [@relusc](https://github.com/relusc)
