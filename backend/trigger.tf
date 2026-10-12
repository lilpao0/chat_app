resource "google_cloud_build_trigger" "chat_api_deploy" {
  name     = "chat-api-deploy"
  location = "asia-southeast1"
  project  = "vinai-lab"

  github {
    owner = "lilpao0"
    repo  = "chat_app"
    push {
      branch = "^main$"
    }
  }

  filename        = "backend/cloudbuild.yaml"
  service_account = "projects/vinai-lab/serviceAccounts/chat-media-builder@vinai-lab.iam.gserviceaccount.com"
  substitutions = {
    _SHA = "$${SHORT_SHA}"
  }
}
