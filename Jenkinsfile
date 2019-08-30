node {
  stage('Checkout scm') {
    checkout scm
  }

  stage('Build Docker image') {
    def img = docker.build('gobac')
  }

  stage('Push Docker image') {
    docker.withRegistry('https://registry.zyra.ca') {
        if (env.BRANCH_NAME == 'master') {
          img.push('latest')
        }
    }
  }
}