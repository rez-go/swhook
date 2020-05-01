# swhook

A simple tool to automate deployment update of a Docker Swarm stack by
utilizing webhook.

**WARNING**: Experimental project.

## Overview

The idea is that we want to ensure that every changes to a stack is recorded.

To make this work, it requires these things:

  - Limit direct access to the stack
  - Use a version control to track changes of the compose file
  - Make the stack be updated everytime there's changes in the compose file's version control system

This tool was designed to handle the 3rd point.

This tool utilizes webhook to listen to push event from a repository, where the compose file is tracked, and
then pull the updated compose file which then to be used as an input to update
the stack.
