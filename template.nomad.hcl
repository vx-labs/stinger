job "vespiary" {
  datacenters = ["dc1"]
  type        = "service"

  update {
    max_parallel     = 1
    min_healthy_time = "30s"
    healthy_deadline = "3m"
    auto_revert      = true
    canary           = 1
  }

  group "vespiary" {
    vault {
      policies      = ["nomad-tls-storer"]
      change_mode   = "signal"
      change_signal = "SIGUSR1"
      env           = false
    }

    count = 3

    restart {
      attempts = 10
      interval = "5m"
      delay    = "15s"
      mode     = "delay"
    }

    ephemeral_disk {
      size = 2000
    }

    task "vespiary-service" {
      shutdown_delay = "15s"
      driver = "docker"

      env {
        HTTPS_PROXY="http://http.proxy.discovery.fr-par.vx-labs.net:3128"
        CONSUL_HTTP_ADDR = "$${NOMAD_IP_rpc}:8500"
        VAULT_ADDR       = "http://active.vault.service.consul:8200/"
      }

      template {
        change_mode = "restart"
        destination = "local/environment"
        env         = true

        data = <<EOH
{{with secret "secret/data/vx/mqtt"}}
VESPIARY_RPC_TLS_CERTIFICATE_FILE="{{ env "NOMAD_TASK_DIR" }}/cert.pem"
VESPIARY_RPC_TLS_PRIVATE_KEY_FILE="{{ env "NOMAD_TASK_DIR" }}/key.pem"
VESPIARY_RPC_TLS_CERTIFICATE_AUTHORITY_FILE="{{ env "NOMAD_TASK_DIR" }}/ca.pem"
no_proxy="10.0.0.0/8,172.16.0.0/12,*.service.consul"
{{end}}
        EOH
      }

      template {
        change_mode = "restart"
        destination = "local/cert.pem"
        splay       = "1h"

        data = <<EOH
{{- $cn := printf "common_name=%s" (env "NOMAD_ALLOC_ID") -}}
{{- $ipsans := printf "ip_sans=%s" (env "NOMAD_IP_rpc") -}}
{{- $sans := printf "alt_names=auth.iot.cloud.vx-labs.net" -}}
{{- $path := printf "pki/issue/grpc" -}}
{{ with secret $path $cn $ipsans $sans "ttl=48h" }}{{ .Data.certificate }}{{ end }}
EOH
      }

      template {
        change_mode = "restart"
        destination = "local/key.pem"
        splay       = "1h"

        data = <<EOH
{{- $cn := printf "common_name=%s" (env "NOMAD_ALLOC_ID") -}}
{{- $ipsans := printf "ip_sans=%s" (env "NOMAD_IP_rpc") -}}
{{- $sans := printf "alt_names=auth.iot.cloud.vx-labs.net" -}}
{{- $path := printf "pki/issue/grpc" -}}
{{ with secret $path $cn $ipsans $sans "ttl=48h" }}{{ .Data.private_key }}{{ end }}
EOH
      }

      template {
        change_mode = "restart"
        destination = "local/ca.pem"
        splay       = "1h"

        data = <<EOH
{{- $cn := printf "common_name=%s" (env "NOMAD_ALLOC_ID") -}}
{{- $ipsans := printf "ip_sans=%s" (env "NOMAD_IP_rpc") -}}
{{- $sans := printf "alt_names=auth.iot.cloud.vx-labs.net" -}}
{{- $path := printf "pki/issue/grpc" -}}
{{ with secret $path $cn $ipsans $sans "ttl=48h" }}{{ .Data.issuing_ca }}{{ end }}
EOH
      }

      config {
        logging {
          type = "fluentd"

          config {
            fluentd-address = "localhost:24224"
            tag             = "vespiary"
          }
        }

        image = "${service_image}:${service_version}"
        args = [
          "--data-dir", "$${NOMAD_TASK_DIR}",
          "--mtls",
          "--raft-bootstrap-expect", "3",
          "--consul-join",
          "--consul-service-name", "vespiary",
          "--consul-service-tag", "gossip",
          "--metrics-port", "8089",
          "--raft-advertized-address", "$${NOMAD_IP_rpc}", "--raft-advertized-port", "$${NOMAD_HOST_PORT_rpc}",
          "--serf-advertized-address", "$${NOMAD_IP_gossip}", "--serf-advertized-port", "$${NOMAD_HOST_PORT_gossip}",
          "--audit-recorder", "grpc",
          "--audit-recorder-grpc-address", "messages.iot.cloud.vx-labs.net:443",
        ]
        force_pull = true

        port_map {
          gossip  = 3799
          rpc     = 3899
          metrics = 8089
        }
      }

      resources {
        cpu    = 200
        memory = 256

        network {
          mbits = 10
          port "rpc" {}
          port "gossip" {}
          port "metrics" {}
        }
      }

      service {
        name = "vespiary"
        port = "rpc"
        tags = [
          "rpc",
          "${service_version}",
          "traefik.enable=true",
          "traefik.tcp.routers.vespiary.rule=HostSNI(`auth.iot.cloud.vx-labs.net`)",
          "traefik.tcp.routers.vespiary.entrypoints=https",
          "traefik.tcp.routers.vespiary.service=vespiary",
          "traefik.tcp.routers.vespiary.tls",
          "traefik.tcp.routers.vespiary.tls.passthrough=true",
        ]
      }
      service {
        name = "vespiary"
        port = "gossip"
        tags = [
          "gossip",
          "${service_version}",
        ]
      }
    }
  }
}
