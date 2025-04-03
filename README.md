# OpenUEM Agent

This repository contains the source code for the OpenUEM Agent

Agents are responsible for gathering and reporting information about the endpoints to OpenUEM. Agents are also responsible for providing an SFTP server and a VNC proxy.

Agents send its reports to Agents workers using NATS messages. Agents workers will store the information in the database

When an agent is installed on an endpoint, it will remain in a "Waiting for admission" state until an administrator validates that this agent can be managed from OpenUEM. When the agent is admitted, a digital certificate is provided to the agent so it can secure its services.

An agent can be disabled if we don't want to receive new reports.

Agents use several digital certificates, and associated private keys, to perform their tasks:

- To authenticate against the NATS servers so the agents can send their reports (agent.cer and agent.key that lives in the certificates folder)
- To secure SFTP and VNC/RDP communications (server.cer and server.key that lives in the certificates folder)
- To authenticate SFTP connections from the console (sftp.cer that lives in the certificates folder)
- To validate certificates signed by our own Certificate Authority (ca.cer)

Agents uses WinGet to install/uninstall packages and configure some settings (registry, local user, local groups...)

OpenUEM console shows the information saved by the workers.

![OpenUEM Console](https://openuem.eu/assets/images/agents_list-37a01f840c883ea4f98f30a199845953.png)

Right now OpenUEM only provides agents for Windows, but soon Linux agents will be released.