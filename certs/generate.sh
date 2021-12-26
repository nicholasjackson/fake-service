#!/bin/bash -e

if [[ $1 = "cleanup" ]]; then
  rm -rf 1_root
  rm -rf 2_intermediate
  rm -rf 3_application
  rm -rf 4_client
  
  exit 0
fi

if [[ $1 = "" ]]; then
  echo "please specify a domain ./generate.sh www.example.com"
  exit 1
fi

if [[ $2 = "" ]]; then
  echo "please specify a password for the private key"
  exit 1
fi


echo 
echo Generate the root key
echo ---
mkdir -p 1_root/private
openssl genrsa -aes256 -passout pass:$2 -out 1_root/private/ca.key.pem 4096

chmod 444 1_root/private/ca.key.pem


echo 
echo Generate the root certificate
echo ---
mkdir -p 1_root/certs
mkdir -p 1_root/newcerts
touch 1_root/index.txt
echo "100212" > 1_root/serial
openssl req -config openssl.cnf \
      -key 1_root/private/ca.key.pem \
      -passin pass:$2 \
      -new -x509 -days 7300 -sha256 -extensions v3_ca \
      -subj "/C=US/ST=Denial/L=Springfield/O=Dis/CN=$1" \
      -out 1_root/certs/ca.cert.pem


echo 
echo Verify root key
echo ---
openssl x509 -noout -text -in 1_root/certs/ca.cert.pem

echo 
echo Generate the key for the intermediary certificate
echo ---
mkdir -p 2_intermediate/private
openssl genrsa -aes256 \
  -passout pass:$2 \
  -out 2_intermediate/private/intermediate.key.pem 4096

chmod 444 2_intermediate/private/intermediate.key.pem


echo 
echo Generate the signing request for the intermediary certificate
echo ---
mkdir -p 2_intermediate/csr
openssl req -config openssl.cnf -new -sha256 \
      -passin pass:$2 \
      -subj "/C=US/ST=Denial/L=Springfield/O=Dis/CN=$1" \
      -key 2_intermediate/private/intermediate.key.pem \
      -out 2_intermediate/csr/intermediate.csr.pem


echo 
echo Sign the intermediary
echo ---
mkdir -p 2_intermediate/certs
mkdir -p 2_intermediate/newcerts
touch 2_intermediate/index.txt
echo "100212" > 2_intermediate/serial
openssl ca -config openssl.cnf -extensions v3_intermediate_ca \
        -passin pass:$2 \
        -days 3650 -notext -md sha256 \
        -in 2_intermediate/csr/intermediate.csr.pem \
        -out 2_intermediate/certs/intermediate.cert.pem

chmod 444 2_intermediate/certs/intermediate.cert.pem


echo 
echo Verify intermediary
echo ---
openssl x509 -noout -text \
      -in 2_intermediate/certs/intermediate.cert.pem

openssl verify -CAfile 1_root/certs/ca.cert.pem \
      2_intermediate/certs/intermediate.cert.pem


echo 
echo Create the chain file
echo ---
cat 2_intermediate/certs/intermediate.cert.pem \
      1_root/certs/ca.cert.pem > 2_intermediate/certs/ca-chain.cert.pem

chmod 444 2_intermediate/certs/ca-chain.cert.pem


echo 
echo Create the application key
echo ---
mkdir -p 3_application/private
openssl genrsa \
      -passout pass:$2 \
    -out 3_application/private/$1.key.pem 2048

chmod 444 3_application/private/$1.key.pem


echo 
echo Create the application signing request
echo ---
mkdir -p 3_application/csr
openssl req -config intermediate_openssl.cnf \
      -subj "/C=US/ST=Denial/L=Springfield/O=Dis/CN=$1" \
      -passin pass:$2 \
      -key 3_application/private/$1.key.pem \
      -new -sha256 -out 3_application/csr/$1.csr.pem


echo 
echo Create the application certificate
echo ---
mkdir -p 3_application/certs
openssl ca -config intermediate_openssl.cnf \
      -passin pass:$2 \
      -extensions server_cert -days 375 -notext -md sha256 \
      -in 3_application/csr/$1.csr.pem \
      -out 3_application/certs/$1.cert.pem

chmod 444 3_application/certs/$1.cert.pem


echo 
echo Validate the certificate
echo ---
openssl x509 -noout -text \
      -in 3_application/certs/$1.cert.pem


echo 
echo Validate the certificate has the correct chain of trust
echo ---
openssl verify -CAfile 2_intermediate/certs/ca-chain.cert.pem \
      3_application/certs/$1.cert.pem


echo 
echo Create the chain file
echo ---
cat 3_application/certs/$1.cert.pem \
      2_intermediate/certs/ca-chain.cert.pem > 3_application/certs/ca-chain.cert.pem

chmod 444 2_application/certs/ca-chain.cert.pem

echo
echo Generate the client key
echo ---
mkdir -p 4_client/private
openssl genrsa \
    -passout pass:$2 \
    -out 4_client/private/$1.key.pem 2048

chmod 444 4_client/private/$1.key.pem


echo
echo Generate the client signing request
echo ---
mkdir -p 4_client/csr
openssl req -config intermediate_openssl.cnf \
      -subj "/C=US/ST=Denial/L=Springfield/O=Dis/CN=$1" \
      -passin pass:$2 \
      -key 4_client/private/$1.key.pem \
      -new -sha256 -out 4_client/csr/$1.csr.pem


echo 
echo Create the client certificate
echo ---
mkdir -p 4_client/certs
openssl ca -config intermediate_openssl.cnf \
      -passin pass:$2 \
      -extensions usr_cert -days 375 -notext -md sha256 \
      -in 4_client/csr/$1.csr.pem \
      -out 4_client/certs/$1.cert.pem

chmod 444 4_client/certs/$1.cert.pem