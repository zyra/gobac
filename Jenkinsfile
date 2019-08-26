node {
  checkout scm

  docker.withRegistry('https://registry.tor1.zyra.ca:5000') {
    def img = docker.build('gobac')

    if (env.BRANCH_NAME == 'master') {
      img.push('latest')
    }
  }
}