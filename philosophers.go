package main

import (
	"fmt"
	"reflect"
	"sync"
	"time"
)

type Chops struct {
	sync.Mutex
}

const numPhilo = 5
const numMeals = 3

type Philo struct {
	id              int
	eaten           int
	leftCS, rightCS *Chops
	ch              chan int
}
type SafeQueue struct {
	queue []int
	mux   sync.Mutex
}

func createChopSticks(numPhilo int) []*Chops {
	CSticks := make([]*Chops, 0, numPhilo)
	for i := 0; i < numPhilo; i++ {
		CSticks = append(CSticks, new(Chops))
	}
	return CSticks
}
func createPhilos(numPhilo int, CSticks []*Chops) []Philo {
	philos := make([]Philo, 0, numPhilo)
	for i := 0; i < numPhilo; i++ {
		philos = append(philos,
			Philo{
				id:      i + 1,
				eaten:   0,
				leftCS:  CSticks[i],
				rightCS: CSticks[(i+1)%numPhilo],
				ch:      make(chan int),
			})
	}
	return philos
}
func removeFirstElementOfQueue(Queue *SafeQueue, withLock bool) int {
	if withLock {
		Queue.mux.Lock()
		defer Queue.mux.Unlock()
	}
	res := Queue.queue[0]
	Queue.queue = Queue.queue[1:]
	return res
}

func safelyAddPhiloToQueue(p *Philo, safeQueue *SafeQueue) {
	safeQueue.mux.Lock()
	safeQueue.queue = append(safeQueue.queue, p.id)
	safeQueue.mux.Unlock()
}
func (p Philo) requestPermissionFromHost(host chan int) {
	host <- p.id
}

func (p Philo) waitForHostToGrantPermission() {
	<-p.ch
}
func (p *Philo) notifyHostEatingIsDone() {
	p.ch <- 2
}

func mapPhiloIdToPhiloIndex(id int) int { return id - 1 }
func createSelectChannels(philos []Philo) []reflect.SelectCase {
	numChannels := len(philos)
	allChannelsSlice := make([]reflect.SelectCase, 0, numChannels)
	for i := 0; i < numChannels; i++ {
		allChannelsSlice = append(allChannelsSlice,
			reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(philos[i].ch),
			},
		)
	}
	return allChannelsSlice
}

func removeFromEatingQueueAfterEatingFinished(p *Philo, eatingQueue *SafeQueue, hostchannel chan int) {
	eatingQueue.mux.Lock()
	defer eatingQueue.mux.Unlock()
	fmt.Println(p.id, eatingQueue.queue)

	pid := p.id
	for idx, v := range eatingQueue.queue {
		if v == pid {
			eatingQueue.queue = append(eatingQueue.queue[0:idx], eatingQueue.queue[idx+1:]...)
			return
		}
	}
	panic("Could not empty the queue")

}

func (p *Philo) eatWithPermission(host chan int) {
	p.requestPermissionFromHost(host)
	p.waitForHostToGrantPermission()
	p.leftCS.Lock()
	p.rightCS.Lock()
	fmt.Println("Start eating", p.id)
	time.Sleep(time.Millisecond * 1000)
	p.eaten++
	p.rightCS.Unlock()
	p.leftCS.Unlock()
	fmt.Println("finished eating", p.id, ";eaten:", p.eaten, "times")
	p.notifyHostEatingIsDone()
}

func (p Philo) grantPhiloPermissiotoEat() {
	p.ch <- 1
}

func (p Philo) philoRoutine(hostChannel chan int, wg *sync.WaitGroup) {
	for p.eaten != 3 {
		p.eatWithPermission(hostChannel)
	}
	wg.Done()
}

func BringFromWaitingQueueToEatingQueue(philos []Philo, waitingQueue, eatingQueue *SafeQueue) {
	for {
		time.Sleep(10 * time.Millisecond)
		waitingQueue.mux.Lock()
		for thereIsRoomForPhiloToEat(eatingQueue) && len(waitingQueue.queue) > 0 {
			pid := removeFirstElementOfQueue(waitingQueue, false)
			philoIdx := mapPhiloIdToPhiloIndex(pid)
			philos[philoIdx].grantPhiloPermissiotoEat()
		}
		waitingQueue.mux.Unlock()
	}

}
func EmptyEatingQueueTask(philos []Philo, eatingQueue *SafeQueue, hostchannel chan int) {
	fmt.Println("Empty Eating Queue task started")
	allChannelsSlice := createSelectChannels(philos)
	for {
		PhiloIndexWhoFinishedEating, _, _ := reflect.Select(allChannelsSlice)
		removeFromEatingQueueAfterEatingFinished(&philos[PhiloIndexWhoFinishedEating], eatingQueue, hostchannel)

	}

}
func thereIsRoomForPhiloToEat(eatingQueue *SafeQueue) bool {
	eatingQueue.mux.Lock()
	defer eatingQueue.mux.Unlock()
	return len(eatingQueue.queue) < 3
}

func HandleIncomingRequests(hostChannel chan int, philos []Philo, waitingQueue *SafeQueue, wg *sync.WaitGroup) {
	for pid := range hostChannel {
		requestigIndex := mapPhiloIdToPhiloIndex(pid)
		safelyAddPhiloToQueue(&philos[requestigIndex], waitingQueue)
	}
	wg.Done()
}

func hostRoutine(hostChannel chan int, philos []Philo, waitingQueue, eatingQueue *SafeQueue, wg *sync.WaitGroup) {
	go BringFromWaitingQueueToEatingQueue(philos, waitingQueue, eatingQueue)
	go EmptyEatingQueueTask(philos, eatingQueue, hostChannel)
	go HandleIncomingRequests(hostChannel, philos, waitingQueue, wg)

}

func disPatchPhilos(philos []Philo, wg_philo *sync.WaitGroup, hostChannel chan int) {
	for _, philo := range philos {
		wg_philo.Add(1)
		go philo.philoRoutine(hostChannel, wg_philo)
	}
}

func main() {
	CSticks := createChopSticks(numPhilo)
	philos := createPhilos(numPhilo, CSticks)
	hostChannel := make(chan int)
	var wg_philo sync.WaitGroup
	var wg_host sync.WaitGroup

	var eatingQueue, waitingQueue SafeQueue

	eatingQueue.queue = make([]int, 0, numPhilo)
	disPatchPhilos(philos, &wg_philo, hostChannel)
	wg_host.Add(1)
	go hostRoutine(hostChannel, philos, &eatingQueue, &waitingQueue, &wg_host)
	wg_philo.Wait()
	wg_host.Wait()
	close(hostChannel)

}
