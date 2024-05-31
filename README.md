Resposta do Lab de Open Telemetry da pós Go Expert.

Para execução dos serviços, rodar o seguinte comando na raiz do projeto:

docker-compose up -d 
Para testar utilize:
curl -X GET "http://localhost:8080/weather?cep={{CEP}}"
Exemplo
curl -X GET "http://localhost:8080/weather?cep=01001000"

Para visualizar os traces, acessar o serviço do zipkin no endereço: http://localhost:9411/ e apertar o botão Run Query.
