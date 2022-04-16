import './web3.min.js';

const web3 = new Web3();


const ethereumButton = document.getElementById('noVNC_metamask_button');
ethereumButton.addEventListener('click', () => {
  getAccount();
});

async function getAccount() {
  const accounts = await ethereum.request({ method: 'eth_requestAccounts' });
  console.log(accounts)
  const chainId = await ethereum.request({ method: 'eth_chainId' })
  console.log(chainId)
  buyMeACaffe(accounts[0])
}


function buyMeACaffe(addr){
  var params = [
    {
      from: addr,
      to: '0x929a9fe76250e14139fac81fb81ba9e9415994a9',
      value: web3.utils.toHex(web3.utils.toWei("0.01"))
    }]
  
  ethereum
    .request({
      method: 'eth_sendTransaction',
      params,
    })
    .then((result) => {
     alert("Tanks! Guy.")
    })
    .catch((error) => {
      console.log(error.message)
    });
}
