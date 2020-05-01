# swhook

A simple tool to automate deployment updates of a Docker Swarm stack by
utilizing webhook.

**WARNING**: Experimental project.

![build](https://github.com/rez-go/swhook/workflows/Build/badge.svg)

## Overview

The idea is that we want to ensure that every change to a stack is tracked.

To make this work, it requires a set up which outlined in these points:

  - Limit direct access to the stack
  - Track the compose file using version control system
  - Make the stack be updated for any change in the compose file

This tool was designed to handle the 3rd point.

This tool utilizes webhook to listen to push event from a repository, where
the compose file is tracked, and then pull the updated compose file which then
to be used as an input to update the stack.
