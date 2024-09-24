# ![OpenUEM Agent for Windows](https://res.cloudinary.com/dyjqffeuz/image/upload/v1722080061/banner_bedozh.png)

This repository contains the source code for the OpenUEM Agent for Windows

Information is retrieved from the Windows Operating System using mainly queries to WMI

## NATS

The agent sends its report information to a NATS server so the available workers treat and save that information to the database

## GRPC

The console uses GRPC to send commands to the agent:

- enable
- disable
- send a report now
- uninstall the agent
