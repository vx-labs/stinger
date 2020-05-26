provider "nomad" {
  version = "~> 1.4"
}
variable image_repository {
  default = "vxlabs/vespiary"
}
variable image_tag {
    default = "latest"
}

resource "nomad_job" "auth" {
  jobspec = templatefile("${path.module}/template.nomad.hcl",
    {
      service_image        = var.image_repository,
      service_version        = var.image_tag,
    },
  )
}
