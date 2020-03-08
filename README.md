# Healthcheck agent

## Description

Script runner that is meant to be used with Auto Scaling to determine the health and set the custom health attribute. At least that is what I built it for. Feel free to use it how you want, I'm sure there are other use cases for it.

The `health checks` can be any process that runs and returns a meaningful exit code. It does not support long running processes. An exit code of 0 is considered good and anything else is a failure.

Logging from processes is considered a warning on STDOUT and an error on STDERR. No attempt is made to determine the actual level, rather the agent is opinionated, successful events should produce no logs, information about potential problems come on STDOUT and errors come on STDERR.

Health checks can fail and recover, recovery is determined by a series of successful runs to stop flapping failures. This is configurable. By default a single failure is considered a stable failure.

Stable failures are health checks that have failed enough times in a row to break the rules in the configuration passed in.

Once a stable failure is found then the Failure hooks are started and all health checking stops. It stops health checking as there is no way to return to healthy from a stable failure.
Examples of failure hooks uses could be to set the custom health attribute of the auto scaling group, drain the server of containers, ship logs, deregister from 3rd party services, basically it just runs a process and expects it to exit with 0. Failure hooks can be retried if they fail, it will give up after the number of configured retries and move onto the next hook in the list.

Health checks are run independently of each other. Therefore it is not uncommon to have 1 check run every 2 seconds and have another run ever 30 seconds. If the failure on the first becomes stable you may never even see a check on the second check.

Failure hooks run sequentially as there may be a need for context between the runs. For example, set the health of the Auto Scaling instance, drain the instances on ECS tasks, wait till draining is complete, then finally, send a complete signal on the Life cycle hook.

## Metrics and status information

There is also a web server to give an overview of the processes configured, the last time it ran and exit codes. The binaries and arguments are not shown as they could have sensitive information in them.
If the server starts failing the overall health will go to `unheathy` and the web server will respond with a 500 to signal that it is not healthy.

The web server only has a single page: `_status`. Everything else will give you a 404.

StatsD is built in to give a heartbeat and run details for every run. These can be used to see the runs passing and failing. It will also fire an event once a stable failure is found.

Lastly, Grace periods.

When the service/app starts it will not consider failures that happen during the grace period which is set in the configuration file. This is used to allow your services start and bootstrapping to happen before the health checks determine the actual health of the server.

## Why use this over a lambda?

Lambdas are great but they grow exponentially when used in this way. For ever server you have you need to have a lambda connect to it and do the check(s). Then you need to handle failures. You need to store the state of that server to detect stable failures. The list of difficulties goes on and on. Rather it would be better to have the server report it's own health and maybe use a failure hook to trigger lambdas to do external work like de-registration. Or better, use the Auto Scaling LifeCycle hooks to trigger lambdas using SNS and SQS. Servers do not need to be left running while you de-register them from most external services. So use a mixture of both where they work best.

## Configuration File

The configuration file is a simple JSON file that is read in once the service starts. If you rewrite the configuration file, you must restart the service to pick up the new configuration.

You can see an example configuration below. Using the -c and -s flags you can point the health checker to a configuration file and see the full working config that you get when the defaults are merged in. Useful if the health checker is not doing what you expect it to.

Example configuration:

```json
{
  "health_checks": [
    {
      "name": "hc1",
      "description": "Test health check",
      "command": "/bin/bash",
      "arguments": [
        "./testscript.sh",
        "0"
      ],
      "frequency_in_seconds": 2,
      "allowed_failures": 2,
      "recovery_success_count": 2
    },
    {
      "name": "control file",
      "description": "reads control file and determines if it must fail",
      "command": "/bin/bash",
      "arguments": [
        "./readfile.sh",
        "./controlFile.txt",
        "aa5aa23fed587045b0a3123b442549c09d52cd68ab762fbda3766f3f94476561"
      ],
      "frequency_in_seconds": 2,
      "allowed_failures": 0,
      "recovery_success_count": 0
    }
  ],
  "failure_hooks": [
    {
      "name": "Failure hook 1",
      "description": "Test failure hook",
      "command": "/bin/bash",
      "arguments": [
        "./testscript.sh",
        "1"
      ],
      "max_retry": 6,
      "seconds_between_retries": 2
    }
  ],
  "startup_grace_seconds": 5,
  "run_failure_hooks_on_term_signal": false,
  "run_failure_hooks": true,
  "exit_after_failure_hooks": true,
  "pretty_logs": true,
  "debug_logging": true,
  "logging_attributes": {
    "hostname": "pickle"
  },
  "webserver": {
    "enabled": true,
    "address": "0.0.0.0",
    "port": 8011,
    "use_tls": false,
    "cert_path": "",
    "key_path": "",
    "pretty_json_responses": true
  },
  "statsd": {
    "enabled": true,
    "address": "127.0.0.1",
    "port": 8125,
    "prefix": "asg_healthcheck",
    "default_tags": {
      "source": "pickle"
    }
  }
}
```

Rather than list out ever configuration item here, you can look in the ./config/config.go file to see them. This save both of us anguish if I forget to mention one of them here.

## Running the agent

The agent can be run as a service and will install/uninstall itself if you use the `-service [install|uninstall]` flag. It supports most linux distros. It should work on windows also but is not tested, be sure to change the location of the configuration file if using windows.
If you need to set a custom configuration file location when installing it as a service, put the flags in when you install the service. eg: `-service install -c /location/to/file.json`

The agent can also run as a stand alone app by simply calling the binary, useful if you wish to run it in a container.

In both instances, the agent will stop at the end of the failure hooks, so keep this in mind when assigning restart policies on both containers and services.

## Help menu

```text
  -c string
        Location of the configuration file. (default "/etc/asg-healthchecker/config.json")
  -h    Shows the help menu.
  -s    Show full running config
  -service string
        Control the system service.
  -v    Outputs the version of the program.
```