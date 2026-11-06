import process from 'node:process'
import { createServer } from 'vite'

function ignoreBrokenPipe(stream) {
  stream.on('error', (error) => {
    if (error?.code === 'EPIPE') {
      return
    }
    throw error
  })
}

ignoreBrokenPipe(process.stdout)
ignoreBrokenPipe(process.stderr)

const server = await createServer()
await server.listen()
server.printUrls()

async function closeServer() {
  await server.close()
  process.exit(0)
}

process.on('SIGINT', closeServer)
process.on('SIGTERM', closeServer)
