# nodectl

The `nodectl` is a command-line interface for managing Darknodes on Ren. It is installed on your local workspace, and will automatically create and update machines for you. Currently it supports **macOS** and **Linux**.

## Installing the tool

To download and install `nodectl`, open a terminal and run:

```sh
curl https://s3.ap-southeast-1.amazonaws.com/darknode.renproject.io/install.sh -sSfL | sh
```

This will download the required binaries and install them to the `$HOME/.nodectl` directory. Open a new terminal to begin using nodectl.

## Updating the tool

**Before updating `nodectl`, please make sure you do not have the tool running in any terminal.**

To update your `nodectl`, open a terminal and run:

```sh
curl https://s3.ap-southeast-1.amazonaws.com/darknode.renproject.io/update.sh -sSfL | sh
```

This will update your `nodectl` to the latest version without affecting any of your deployed nodes.

> Note: make sure you are using Terraform version > 1.0.0 ! To upgrade Terraform, download the executable for your operating system from https://www.terraform.io/downloads.html and copy it to `$HOME/.nodectl/bin/terraform`.


## Usage

### Deploy a Darknode

#### AWS

To deploy a Darknode on AWS, open a terminal and run:

```sh
nodectl up --name my-first-darknode --aws --aws-access-key YOUR-AWS-ACCESS-KEY --aws-secret-key YOUR-AWS-SECRET-KEY
``` 

The `nodectl` will automatically use the credentials available at `$HOME/.aws/credentials` if you do not explicitly set the `--access-key` and `--secret-key` arguments.
By default, it will use the credentials of `default` profile.

You can also specify the region and instance type you want to use for the Darknode:

```sh
nodectl up --name my-first-darknode --aws --aws-access-key YOUR-AWS-ACCESS-KEY --aws-secret-key YOUR-AWS-SECRET-KEY --aws-region eu-west-1 --aws-instance t2.small
``` 
The default instance type is `t3.micro` and region will be random.
You can find all available regions and instance types at [AWS](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/Concepts.RegionsAndAvailabilityZones.html).

#### Digital Ocean

You first need to create an API token.
To deploy a Darknode on Digital Ocean, open a terminal and run:

```sh
nodectl up --name my-first-darknode --do --do-token YOUR-API-TOKEN
``` 

You can also specify the region and droplet size you want to use for the Darknode:

```sh
nodectl up --name my-first-darknode --do --do-token YOUR-API-TOKEN --do-region nyc1 --do-droplet s-2vcpu-2gb
``` 

The default droplet size is `s-1vcpu-1gb` and region will be random.
Be aware some region and droplet size are not available to all users.

You can find all available regions and droplet size slug by using the digital ocean [API](https://developers.digitalocean.com/documentation/v2/#regions).

#### Testnet

If you want to join RenVM testnet instead of mainnet, you can specify the network you want to join when 
deploying the node

```shell

#  For AWS 
nodectl up --name my-first-darknode --aws --aws-access-key YOUR-AWS-ACCESS-KEY --aws-secret-key YOUR-AWS-SECRET-KEY --network testnet

#  For digital ocean
nodectl up --name my-first-darknode --do --do-token YOUR-API-TOKEN --network testnet

```

### Destroy a Darknode

_**WARNING: Before destroying a Darknode make sure you have de-registered it, and withdrawn all fees earned! You will not be able to destroy your darknode if it's not fully deregistered. The CLI will guide you to the page where you can deregister your node**_

Destroying a Darknode will turn it off and tear down all resources allocated by the cloud provider. To destroy a Darknode, open a terminal and run:

```sh
nodectl destroy my-first-darknode
``` 

To avoid the command-line prompt confirming the destruction, use the `--force` argument:

```sh
nodectl destroy --force my-first-darknode 
```

We do not recommend using the `--force` argument unless you are developing custom tools that manage your Darknodes automatically.

### Get Darknode's peer address
To get the Darknode's peer address, open a terminal and run:

```sh
nodectl address my-first-darknode 
```
You can send your Darknode's peer address to others to be included in other Darknode's config files.

### List all Darknodes

The `nodectl` supports deploying multiple Darknodes. To list all available Darknodes, open a terminal and run:

```sh
nodectl list
```

### Start/Stop/Restart Darknode

To turn off your darknode, open a terminal and run:

```sh
nodectl stop my-first-darknode

``` 

Note this won't shut down the cloud instance, so you will still be charged by your cloud provider.
If it is already off, `stop` will do nothing.


To turn on your darknode, open a terminal and run:

```sh
nodectl start my-first-darknode
``` 

If it is already on, `start` will do nothing.

To restart your darknode, open a terminal and run:

```sh
nodectl restart my-first-darknode
``` 

### SSH into Darknode

To access your Darknode using SSH, open a terminal and run:

```sh
nodectl ssh my-first-darknode
```

