def buildBin(os, arch) {
  return {
      def output = "gobac-${os}-${arch}"
      sh "GOOS=${os} GOARCH=${arch} go build -mod vendor -o ${output} cmd/gobac/main.go"
      archiveArtifacts(artifacts: output)
  }
}


node {
  stage('Checkout scm') {
    checkout scm
  }

  def img

  stage('Build Docker image') {
    img = docker.build('zyra/gobac')
  }

  stage('Push Docker image') {
    docker.withRegistry('https://harbor.zyra.ca', 'harbor') {
        if (env.BRANCH_NAME == 'master') {
          img.push('latest')
        }
    }
  }

  docker.image('golang:1.12').inside('-u root') {
    def buildConfigs = [
      ['linux', ['amd64', 'arm64', 'arm']],
      ['windows', ['amd64']],
      ['darwin', ['amd64']],
    ]

    def parallelBuilds = [:]

    for (config in buildConfigs) {
      def os = config[0]

      for (arch in config[1]) {
        parallelBuilds["Build gobac-${os}-${arch}"] = buildBin(os, arch)
      }
    }

    parallelBuilds.failFast = true

    stage('Build binaries') {
      parallel parallelBuilds
    }
  }
}