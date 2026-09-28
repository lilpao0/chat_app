resource "google_cloud_build_trigger" "chat_api_deploy" {
  name        = "chat-api-deploy"
  location    = "asia-southeast1"
  project     = "vinai-lab"

  github {
    owner = "lilpao0"
    repo  = "chat_app"
    push {
      branch = "^main$"
    }
  }

  build {
    step {
      name = "gcr.io/cloud-builders/docker"
      args = ["build", "-t", "asia-southeast1-docker.pkg.dev/vinai-lab/chat-repo/chat-api:${SHORT_SHA}", "-t", "asia-southeast1-docker.pkg.dev/vinai-lab/chat-repo/chat-api:latest", "."]
    }
    step {
      name = "gcr.io/cloud-builders/docker"
      args = ["push", "-a", "asia-southeast1-docker.pkg.dev/vinai-lab/chat-repo/chat-api"]
    }
    step {
      name = "gcr.io/google.com/cloudsdktool/cloud-sdk"
      entrypoint = "gcloud"
      args = ["run", "deploy", "chat-api", "--image", "asia-southeast1-docker.pkg.dev/vinai-lab/chat-repo/chat-api:${SHORT_SHA}", "--region", "asia-southeast1", "--platform", "managed", "--allow-unauthenticated", "--port", "8080", "--min-instances", "1", "--timeout", "3600s", "--set-secrets", "JWT_SECRET=jwt-secret:latest,DATABASE_URL=database-url:latest"]
    }
  }
}
