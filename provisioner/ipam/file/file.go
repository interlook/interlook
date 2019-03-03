package file

// TODO: write readme
// TODO: implement stop
import (
	"encoding/json"
	"errors"
	"github.com/bhuisgen/interlook/log"
	"github.com/bhuisgen/interlook/service"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"sync"
)

type Extension struct {
	IPStart     string `yaml:"ip_start"`
	IPEnd       string `yaml:"ip_end"`
	NetworkCidr string `yaml:"network_cidr"`
	DbFile      string `yaml:"db_file"`
	db          db
	config      *config
}

type config struct {
	IPRange net.Addr
}

type IPAMRecord struct {
	IP   string `json:"ip"`
	Host string `json:"host"`
}

type db struct {
	sync.Mutex
	Records []IPAMRecord `json:"records"`
}

func (d db) isIPFree(ip net.IP) bool {
	for _, rec := range d.Records {
		if ip.String() == rec.IP {
			return false
		}
	}
	return true
}

func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func (p *Extension) Start(receive <-chan service.Message, send chan<- service.Message) error {
	// load db from file
	if err := p.db.load(p.DbFile); err != nil {
		logger.DefaultLogger().Warnf("error loading db file %v", err.Error())
	}
	for {
		msg := <-receive
		logger.DefaultLogger().Debugf("ipam.file received message %v\n", msg)

		switch msg.Action {
		case "delete":
			msg.Action = "extUpdate"
			if err := p.db.deleteService(msg.Service.Name); err != nil {
				msg.Error = err.Error()
				logger.DefaultLogger().Error(msg.Error)
				send <- msg
				continue
			}
			msg.Service.DNSName = ""
			msg.Service.PublicIP = ""
			send <- msg
		default:
			// check if service is already defined
			// if yes send back msg with update action
			// if not, get new IPAM, update service def and send back msg
			msg.Action = "extUpdate"

			if p.serviceExist(&msg) {
				logger.DefaultLogger().Debugf("service %v already exist", msg.Service.DNSName)

				record := p.db.getServiceByName(msg.Service.DNSName)
				msg.Service.DNSName = record.Host
				msg.Service.PublicIP = record.IP

				send <- msg
				continue
			}
			logger.DefaultLogger().Debugf("service %v does not exist, adding", msg.Service.DNSName)
			ip, err := p.addService(msg.Service.DNSName)
			if err != nil {
				msg.Error = err.Error()
				send <- msg
				continue
			}
			msg.Service.PublicIP = ip
			send <- msg
		}
	}
}

func (p *Extension) Stop() {

}

func (d *db) deleteService(name string) error {
	for i, v := range d.Records {
		if v.Host == name {
			d.Records = append(d.Records[:i], d.Records[i+1:]...)
			return nil
		}
	}
	return errors.New("Could not find record for " + name)
}

func (d db) getServiceByName(name string) (svc IPAMRecord) {
	for k, v := range d.Records {
		if v.Host == name {
			return d.Records[k]
		}
	}
	return svc
}

func (p Extension) addService(name string) (newIP string, err error) {
	logger.DefaultLogger().Debugf("cidr: %v", p.NetworkCidr)
	ip, ipnet, err := net.ParseCIDR(p.NetworkCidr)
	if err != nil {
		return "", err
	}
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incrementIP(ip) {

		if ip.IsMulticast() || strings.Contains(ipnet.String(), ip.String()) {
			continue
		}
		if p.db.isIPFree(ip) {
			newRec := IPAMRecord{ip.String(), name}
			p.db.Records = append(p.db.Records, newRec)
			if err := p.db.save(p.DbFile); err != nil {
				logger.DefaultLogger().Errorf("Error saving db to %v\n", err)
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("no available IPAM")
}

func (d *db) save(file string) error {
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(d.Records)
	{
		if err != nil {
			return err
		}
	}
	_, err = f.Write(data)
	if err != nil {
		return err
	}
	f.Sync()

	return nil
}

func (d *db) load(file string) error {
	f, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(f, &d.Records); err != nil {
		return err
	}
	return nil
}

func (p *Extension) serviceExist(msg *service.Message) bool {
	for _, v := range p.db.Records {
		if v.Host == msg.Service.DNSName {
			return true
		}
	}
	return false
}
