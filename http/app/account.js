import './web3.min.js';


const ethereumButton = document.getElementById('noVNC_metamask_button');
ethereumButton.addEventListener('click', () => {
  getAccount();
});

async function getAccount() {
  const accounts = await ethereum.request({ method: 'eth_requestAccounts' });
  console.log(accounts)
}
