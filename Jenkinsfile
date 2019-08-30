node {
  stage('Checkout scm') {
    checkout scm
  }

  def img

  stage('Build Docker image') {
    img = docker.build('gobac')
  }

  stage('Push Docker image') {
    docker.withRegistry('https://registry.zyra.ca') {
        if (env.BRANCH_NAME == 'master') {
          img.push('latest')
        }
    }
  }
}