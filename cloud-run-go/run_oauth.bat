@echo off
cd /d k:\.gemini\HomeDocManager\cloud-run-go
set GCP_PROJECT_ID=family-document-manager-486009
"C:\Program Files\Go\bin\go.exe" run tools/setup_oauth.go
