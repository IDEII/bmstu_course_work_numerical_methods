package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	SUSCEPTIBLE       = "Подвержен"
	LATENT            = "Латентный"
	SUBCLINICAL       = "Субклинический"
	HIDDEN_INFECTED   = "Скрыто заражен"
	INFECTED_HOME     = "Заражен (дома)"
	INFECTED_HOSPITAL = "Заражен (стационар)"
	RECOVERED         = "Восстановился"
	IMMUNE            = "Иммунный"
	DECEASED          = "Умер"
	SCHOOL_CHILD      = "Школьник/Дошкольник"
	STUDENT           = "Студент"
	OFFICE_WORKER     = "Работник предприятия/офиса"
	SERVICE_WORKER    = "Работник сферы обслуживания"
	HEALTHCARE_WORKER = "Работник сферы здравоохранения"
	PENSIONER         = "Пенсионер"
)

type Metrics struct {
	ActiveCases int
	TotalDeaths int
}

type Report struct {
	ReportDate  string
	I           int
	D           int
	H           int
	R           int
	TI          int
	TD          int
	TR          int
	ActiveCases string
}

type DataPoint struct {
	Date        time.Time
	ActiveCases int
}

type Params struct {
	infectionRate      float64
	immunityDuration   int
	transitionLatent   float64
	transitionHidden   float64
	transitionHome     float64
	transitionHospital float64
	recoveryChance     float64
	deathChance        float64
}

type Particle struct {
	Position []float64
	Velocity []float64
	BestPos  []float64
	BestEval float64
	Param    Params
}

type Agent struct {
	id              int
	state           string
	daysLatent      int
	daysSubclinical int
	daysInfected    int
	daysRecovered   int
	daysImmune      int
	isAlive         bool
	family          *Family
	workplace       *Workplace
	socialGroup     string
}

type Family struct {
	id     int
	home   *Home
	agents []*Agent
}

type Home struct {
	id       int
	families []*Family
}

type Workplace struct {
	id     int
	Type   string
	agents []*Agent
}

type DiseaseModel struct {
	populationSize     int
	infectionRate      float64
	latentPeriod       int
	subclinicalPeriod  int
	infectiousPeriod   int
	interactionsPerDay int
	numHomes           int
	familiesPerHome    int
	numWorkplaces      int
	days               int
	increment          float64
	workersFraction    float64
	lockdown           bool
	Trust              float64
	homes              []*Home
	families           []*Family
	workplaces         []*Workplace
	population         []*Agent
	history            map[string][]int
	time               int
	immunityDuration   int
	infectionRateLimit float64
	transitionLatent   float64
	transitionHidden   float64
	transitionHome     float64
	transitionHospital float64
	recoveryChance     float64
	deathChance        float64
	All_cases          []int
}

func (a *Agent) expose() {
	if a.state == SUSCEPTIBLE {
		a.state = LATENT
		a.daysLatent = 0
	}
}

func (a *Agent) becomeSubclinical() {
	if a.state == LATENT {
		a.state = SUBCLINICAL
		a.daysSubclinical = 0
	}
}

func (a *Agent) becomeHiddenInfected() {
	if a.state == SUBCLINICAL || a.state == LATENT {
		a.state = HIDDEN_INFECTED
	}
}

func (a *Agent) infectHome() {
	if a.state == SUBCLINICAL || a.state == HIDDEN_INFECTED {
		a.state = INFECTED_HOME
		a.daysInfected = 0
	}
}

func (a *Agent) infectHospital() {
	if a.state == SUBCLINICAL || a.state == HIDDEN_INFECTED {
		a.state = INFECTED_HOSPITAL
		a.daysInfected = 0
	}
}

func (a *Agent) recover() {
	if a.state == INFECTED_HOME || a.state == INFECTED_HOSPITAL {
		a.state = RECOVERED
	}
}

func (a *Agent) becomeImmune() {
	if a.state == RECOVERED {
		a.state = IMMUNE
		a.daysImmune = 0
	}
}

func (a *Agent) die() {
	a.state = DECEASED
	a.isAlive = false
}

func Model(populationSize int, infectionRate float64, latentPeriod, subclinicalPeriod, infectiousPeriod, interactionsPerDay, numHomes, familiesPerHome, numWorkplaces int, workersFraction float64, lockdown bool, immunityDuration int, transitionLatent float64, transitionHidden float64, transitionHome float64, transitionHospital float64, recoveryChance float64, deathChance float64) *DiseaseModel {
	model := &DiseaseModel{
		populationSize:     populationSize,
		infectionRate:      infectionRate,
		latentPeriod:       latentPeriod,
		subclinicalPeriod:  subclinicalPeriod,
		infectiousPeriod:   infectiousPeriod,
		interactionsPerDay: interactionsPerDay,
		numHomes:           numHomes,
		familiesPerHome:    familiesPerHome,
		numWorkplaces:      numWorkplaces,
		workersFraction:    workersFraction,
		lockdown:           lockdown,
		homes:              make([]*Home, numHomes),
		families:           []*Family{},
		workplaces:         []*Workplace{},
		population:         []*Agent{},
		days:               0,
		increment:          0.0001,
		Trust:              0,
		infectionRateLimit: 0.02,
		All_cases:          []int{},
		history: map[string][]int{
			"S":     {},
			"E":     {},
			"SC":    {},
			"HI":    {},
			"IH":    {},
			"IHosp": {},
			"R":     {},
			"Imm":   {},
			"D":     {},
		},
		immunityDuration:   immunityDuration,
		transitionLatent:   transitionLatent,
		transitionHidden:   transitionHidden,
		transitionHome:     transitionHome,
		transitionHospital: transitionHospital,
		recoveryChance:     recoveryChance,
		deathChance:        deathChance,
	}

	workTypes := []string{}
	for i := 0; i < int(0.03*float64(numWorkplaces)); i++ {
		workTypes = append(workTypes, HEALTHCARE_WORKER)
	}
	for i := 0; i < int(0.25*float64(numWorkplaces)); i++ {
		workTypes = append(workTypes, OFFICE_WORKER)
	}
	for i := 0; i < int(0.28*float64(numWorkplaces)); i++ {
		workTypes = append(workTypes, SERVICE_WORKER)
	}
	for len(workTypes) < numWorkplaces {
		workTypes = append(workTypes, OFFICE_WORKER)
	}
	rand.Shuffle(len(workTypes), func(i, j int) {
		workTypes[i], workTypes[j] = workTypes[j], workTypes[i]
	})

	for i := 0; i < numWorkplaces; i++ {
		workplace := &Workplace{id: i, Type: workTypes[i], agents: []*Agent{}}
		model.workplaces = append(model.workplaces, workplace)
	}

	familyID := 0
	for i := 0; i < numHomes; i++ {
		home := &Home{id: i, families: []*Family{}}
		model.homes[i] = home
		for j := 0; j < familiesPerHome; j++ {
			family := &Family{id: familyID, home: home, agents: []*Agent{}}
			model.families = append(model.families, family)
			home.families = append(home.families, family)
			familyID++
		}
	}

	socialGroups := model.assignSocialGroups(populationSize)
	for agentID := 0; agentID < populationSize; agentID++ {
		family := model.families[rand.Intn(len(model.families))]
		socialGroup := socialGroups[agentID]
		agent := &Agent{id: agentID, family: family, socialGroup: socialGroup, state: SUSCEPTIBLE, isAlive: true}
		family.agents = append(family.agents, agent)
		model.population = append(model.population, agent)
	}

	numWorkers := int(float64(populationSize) * workersFraction)
	workers := []*Agent{}

	availableWorkers := []*Agent{}
	for _, agent := range model.population {
		if agent.socialGroup == OFFICE_WORKER || agent.socialGroup == SERVICE_WORKER || agent.socialGroup == HEALTHCARE_WORKER {
			availableWorkers = append(availableWorkers, agent)
		}
	}

	numWorkersToSample := min(numWorkers, len(availableWorkers))
	if numWorkersToSample > 0 {
		workers = sampleAgents(availableWorkers, numWorkersToSample)
	}

	for _, worker := range workers {
		workplace := model.workplaces[rand.Intn(numWorkplaces)]
		worker.workplace = workplace
		workplace.agents = append(workplace.agents, worker)
	}

	return model
}

func (model *DiseaseModel) assignSocialGroups(populationSize int) []string {
	groups := []string{}
	groups = append(groups, make([]string, int(0.13*float64(populationSize)), populationSize)...)
	groups = append(groups, make([]string, int(0.07*float64(populationSize)), populationSize)...)
	groups = append(groups, make([]string, int(0.25*float64(populationSize)), populationSize)...)
	groups = append(groups, make([]string, int(0.28*float64(populationSize)), populationSize)...)
	groups = append(groups, make([]string, int(0.03*float64(populationSize)), populationSize)...)
	groups = append(groups, make([]string, int(0.24*float64(populationSize)), populationSize)...)

	for len(groups) < populationSize {
		groups = append(groups, randomSocialGroup())
	}
	rand.Shuffle(len(groups), func(i, j int) {
		groups[i], groups[j] = groups[j], groups[i]
	})
	return groups[:populationSize]
}

func (model *DiseaseModel) initializeInfected(initialInfected int) {
	initial := sampleAgents(filterAgents(model.population, func(a *Agent) bool { return a.isAlive }), initialInfected)
	for _, agent := range initial {
		agent.expose()
	}
}

func (model *DiseaseModel) step() {
	newExposed := []*Agent{}
	newSubclinical := []*Agent{}
	newHidden := []*Agent{}
	newInfectedHome := []*Agent{}
	newInfectedHospital := []*Agent{}
	newRecovered := []*Agent{}
	newImmune := []*Agent{}
	newDeceased := []*Agent{}
	newSusceptible := []*Agent{}
	model.days += 1
	HiddentoHometransition := 0.15
	immunitylostchance := 0.01
	model.increment = 0.00151
	model.infectionRate += model.increment
	model.transitionHidden -= model.increment
	if model.transitionHidden < 0.6 {
		model.transitionHidden = 0.6
	}

	if model.days >= 2*model.immunityDuration {
		model.Trust += model.increment
	}
	if model.Trust > 0.9 {
		model.Trust = 0.9
	}
	c := 1.0
	fl := 0
	if model.days%model.immunityDuration == 0 && model.days != 0 && model.days-(model.immunityDuration/3) > 0 {
		fl = 1
		c = float64(model.days/model.immunityDuration) + 1.1
		model.increment *= c
	}

	if fl == 1 {
		model.infectionRateLimit = math.Min(model.infectionRateLimit*float64(c), 0.8)
	}
	if len(model.history) > 10 {
		if model.history[INFECTED_HOME][len(model.history)-2]+model.history[INFECTED_HOSPITAL][len(model.history)-2] < model.history[INFECTED_HOME][len(model.history)-1]+model.history[INFECTED_HOSPITAL][len(model.history)-1] {
			model.infectionRateLimit += model.increment
		}
	}

	if model.infectionRate > model.infectionRateLimit {
		model.infectionRate = model.infectionRateLimit
	}

	if model.days%model.immunityDuration == 0 && model.days != 0 && model.days-(model.immunityDuration) > 0 {
		HiddentoHometransition = math.Min(HiddentoHometransition+model.increment, 0.23)
	}
	if model.days == 0 {
		HiddentoHometransition = HiddentoHometransition * 2.0
	}

	if model.days%model.immunityDuration == 0 && model.days != 0 {
		immunitylostchance += model.increment
	}
	if immunitylostchance > 0.05 {
		immunitylostchance = 0.05
	}

	for _, agent := range model.population {
		if !agent.isAlive {
			continue
		}

		switch agent.state {
		case LATENT:
			agent.daysLatent++
			if agent.daysLatent >= model.latentPeriod {
				if rand.Float64() < 0.01 {
					newRecovered = append(newRecovered, agent)
				} else {
					newSubclinical = append(newSubclinical, agent)
				}
			}
		case SUBCLINICAL:
			agent.daysSubclinical++
			if agent.daysSubclinical >= model.subclinicalPeriod {
				if rand.Float64() < 0.01 {
					newRecovered = append(newRecovered, agent)
				} else {
					transition := rand.Float64()
					if transition < model.transitionHidden {
						newHidden = append(newHidden, agent)
					} else if transition < model.transitionHome {
						newInfectedHome = append(newInfectedHome, agent)
					} else {
						newInfectedHospital = append(newInfectedHospital, agent)
					}
				}
			}
		case HIDDEN_INFECTED:
			agent.daysInfected++
			if agent.daysInfected >= model.infectiousPeriod {

				if rand.Float64() < HiddentoHometransition {
					newInfectedHome = append(newInfectedHome, agent)
				} else {
					if rand.Float64() < model.recoveryChance {
						newRecovered = append(newRecovered, agent)
					}
				}
			}
		case INFECTED_HOME:
			agent.daysInfected++
			if agent.daysInfected >= model.infectiousPeriod {
				if rand.Float64() < model.transitionHome && model.days/model.immunityDuration >= 3 {
					newHidden = append(newHidden, agent)
				} else {
					if rand.Float64() < model.recoveryChance {
						newRecovered = append(newRecovered, agent)
					} else {
						newInfectedHospital = append(newInfectedHospital, agent)
					}
				}
			}
		case INFECTED_HOSPITAL:
			agent.daysInfected++
			if agent.daysInfected >= model.infectiousPeriod {
				if rand.Float64() < model.deathChance {
					newDeceased = append(newDeceased, agent)
				} else {
					if rand.Float64() < model.recoveryChance {
						newRecovered = append(newRecovered, agent)
					}
				}
			}
		case IMMUNE:
			agent.daysImmune++

			if agent.daysImmune >= model.immunityDuration || rand.Float64() < immunitylostchance {
				newSusceptible = append(newSusceptible, agent)
			}
		}

		if agent.state == IMMUNE {
			continue
		}

		if agent.state == SUSCEPTIBLE {
			if model.days/model.immunityDuration > 2 {
				vacineChance := rand.Float64()
				if vacineChance < model.Trust {
					newRecovered = append(newRecovered, agent)
				} else {
					continue
				}
			} else {
				continue
			}
		}

		isInfectious := agent.state == SUBCLINICAL || agent.state == HIDDEN_INFECTED || agent.state == INFECTED_HOME || agent.state == INFECTED_HOSPITAL

		if !isInfectious {
			continue
		}

		if agent.state == INFECTED_HOSPITAL {
			continue
		}

		for _, familyMember := range agent.family.agents {
			if familyMember.state == SUSCEPTIBLE {
				if rand.Float64() < model.infectionRate {
					newExposed = append(newExposed, familyMember)
				}
			}
		}
		if agent.state == INFECTED_HOME {
			continue
		}

		if !model.lockdown && agent.workplace != nil {
			for _, coworker := range agent.workplace.agents {
				if coworker.id == agent.id || coworker.state != SUSCEPTIBLE {
					continue
				}
				if rand.Float64() < model.infectionRate*0.1 {
					newExposed = append(newExposed, coworker)
				}
			}
		} else {
			if model.lockdown && agent.socialGroup == HEALTHCARE_WORKER {
				for _, coworker := range agent.workplace.agents {
					if coworker.id == agent.id || coworker.state != SUSCEPTIBLE {
						continue
					}
					if rand.Float64() < model.infectionRate-0.3 {
						newExposed = append(newExposed, coworker)
					}
				}
			}
		}

		if !model.lockdown {
			for i := 0; i < model.interactionsPerDay; i++ {
				target := model.population[rand.Intn(len(model.population))]
				if target.id == agent.id || target.state != SUSCEPTIBLE {
					continue
				}
				if rand.Float64() < model.infectionRate {
					newExposed = append(newExposed, target)
				}
			}
		} else {
			if agent.socialGroup == HEALTHCARE_WORKER {
				for i := 0; i < model.interactionsPerDay; i++ {
					target := model.population[rand.Intn(len(model.population))]
					if target.id == agent.id || target.state != SUSCEPTIBLE {
						continue
					}
					if rand.Float64() < model.infectionRate-0.3 {
						newExposed = append(newExposed, target)
					}
				}
			}
		}

	}

	for _, agent := range newExposed {
		agent.expose()
	}
	for _, agent := range newSubclinical {
		agent.becomeSubclinical()
	}
	for _, agent := range newHidden {
		agent.becomeHiddenInfected()
	}
	for _, agent := range newInfectedHome {
		agent.infectHome()
	}
	for _, agent := range newInfectedHospital {
		agent.infectHospital()
	}
	for _, agent := range newRecovered {
		agent.recover()
		newImmune = append(newImmune, agent)
	}
	for _, agent := range newImmune {
		agent.becomeImmune()
	}
	for _, agent := range newDeceased {
		agent.die()
	}
	for _, agent := range newSusceptible {
		agent.state = SUSCEPTIBLE
	}

	counts := model.countStates()
	for key, _ := range model.history {
		model.history[key] = append(model.history[key], counts[key])
	}

	infectedCount := counts[INFECTED_HOME] + counts[INFECTED_HOSPITAL]
	total := len(model.population)
	percentage := (float64(infectedCount) / float64(total)) * 100
	percentage2 := (float64(counts[IMMUNE]) / float64(total)) * 100
	if !model.lockdown && percentage >= 2 && percentage2 <= 50 {
		model.toggleLockdown(true)
	} else if model.lockdown && percentage <= 20 || percentage2 >= 50 {
		model.toggleLockdown(false)
	}

	model.time++
}

func (model *DiseaseModel) countStates() map[string]int {
	counts := map[string]int{}
	for _, agent := range model.population {
		if !agent.isAlive {
			counts["D"]++
			continue
		}
		switch agent.state {
		case SUSCEPTIBLE:
			counts["S"]++
		case LATENT:
			counts["E"]++
		case SUBCLINICAL:
			counts["SC"]++
		case HIDDEN_INFECTED:
			counts["HI"]++
		case INFECTED_HOME:
			counts["IH"]++
		case INFECTED_HOSPITAL:
			counts["IHosp"]++
		case RECOVERED:
			counts["R"]++
		case IMMUNE:
			counts["Imm"]++
		}
	}
	return counts
}

func (model *DiseaseModel) toggleLockdown(lockdownStatus bool) {
	model.lockdown = lockdownStatus
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func randomSocialGroup() string {
	groups := []string{SCHOOL_CHILD, STUDENT, OFFICE_WORKER, SERVICE_WORKER, HEALTHCARE_WORKER, PENSIONER}
	return groups[rand.Intn(len(groups))]
}

func sampleAgents(agents []*Agent, num int) []*Agent {
	sampled := make([]*Agent, num)
	perm := rand.Perm(len(agents))
	for i := 0; i < num; i++ {
		sampled[i] = agents[perm[i]]
	}
	return sampled
}

func filterAgents(agents []*Agent, filter func(*Agent) bool) []*Agent {
	filtered := []*Agent{}
	for _, agent := range agents {
		if filter(agent) {
			filtered = append(filtered, agent)
		}
	}
	return filtered
}

func pso(data []DataPoint, swarmSize, iterations, dim int, lb, ub []float64) ([]float64, Params) {
	rand.Seed(time.Now().UnixNano())

	swarm := make([]Particle, swarmSize)
	globalBest := make([]float64, dim)
	globalBestEval := math.MaxFloat64
	var globalBestParam Params

	for i := 0; i < swarmSize; i++ {
		swarm[i].Position = make([]float64, dim)
		swarm[i].Velocity = make([]float64, dim)
		swarm[i].BestPos = make([]float64, dim)
		swarm[i].BestEval = math.MaxFloat64

		for d := 0; d < dim; d++ {
			if d == 0 {
				swarm[i].Position[d] = lb[d] + rand.Float64()*(ub[d]-lb[d])
			} else {
				swarm[i].Position[d] = lb[d] + rand.Float64()*(ub[d]-lb[d])
			}
			swarm[i].Velocity[d] = 0
			swarm[i].BestPos[d] = swarm[i].Position[d]
		}
		swarm[i].Param = Params{
			infectionRate:      swarm[i].Position[0],
			immunityDuration:   int(swarm[i].Position[1]),
			transitionLatent:   swarm[i].Position[2],
			transitionHidden:   swarm[i].Position[3],
			transitionHome:     swarm[i].Position[4],
			transitionHospital: swarm[i].Position[5],
			recoveryChance:     swarm[i].Position[6],
			deathChance:        swarm[i].Position[7],
		}
	}

	w := 0.729
	c1 := 1.49445
	c2 := 1.49445

	var mutex sync.Mutex

	for it := 0; it < iterations; it++ {
		var wg sync.WaitGroup

		for i := 0; i < swarmSize; i++ {
			wg.Add(1)
			go func(p *Particle) {
				defer wg.Done()
				eval := objectiveFunction(p.Position, data)

				mutex.Lock()
				if eval < p.BestEval {
					p.BestEval = eval
					copy(p.BestPos, p.Position)
				}
				if eval < globalBestEval {
					globalBestEval = eval
					globalBestParam = p.Param
					copy(globalBest, p.Position)
				}
				mutex.Unlock()
			}(&swarm[i])
		}

		wg.Wait()

		for i := 0; i < swarmSize; i++ {
			for d := 0; d < dim; d++ {
				lr1 := rand.Float64()
				lr2 := rand.Float64()

				swarm[i].Velocity[d] = w*swarm[i].Velocity[d] +
					c1*lr1*(swarm[i].BestPos[d]-swarm[i].Position[d]) +
					c2*lr2*(globalBest[d]-swarm[i].Position[d])

				maxVel := (ub[d] - lb[d]) * 0.001
				if swarm[i].Velocity[d] > maxVel {
					swarm[i].Velocity[d] = maxVel
				}
				if swarm[i].Velocity[d] < -maxVel {
					swarm[i].Velocity[d] = -maxVel
				}

				swarm[i].Position[d] += swarm[i].Velocity[d]

				if swarm[i].Position[d] > ub[d] {
					swarm[i].Position[d] = ub[d]
				}
				if swarm[i].Position[d] < lb[d] {
					swarm[i].Position[d] = lb[d]
				}
			}

			swarm[i].Param = Params{
				infectionRate:      swarm[i].Position[0],
				immunityDuration:   int(swarm[i].Position[1]),
				transitionLatent:   swarm[i].Position[2],
				transitionHidden:   swarm[i].Position[3],
				transitionHome:     swarm[i].Position[4],
				transitionHospital: swarm[i].Position[5],
				recoveryChance:     swarm[i].Position[6],
				deathChance:        swarm[i].Position[7],
			}
		}

		fmt.Printf("Итерация %d/%d, Лучшее значение: %f\n", it+1, iterations, globalBestEval)
	}

	return globalBest, globalBestParam
}

func objectiveFunction(params []float64, data []DataPoint) float64 {
	infectionRate := params[0]
	immunityDuration := int(params[1])
	transitionLatent := params[2]
	transitionHidden := params[3]
	transitionHome := params[4]
	transitionHospital := params[5]
	recoveryChance := params[6]
	deathChance := params[7]

	var totalError float64

	populationSize := 10382754
	latentPeriod := 2
	subclinicalPeriod := 2
	infectiousPeriod := 14
	interactionsPerDay := 5
	numHomes := 3808650
	familiesPerHome := 4
	numWorkplaces := 824000
	workersFraction := 0.7
	model := Model(populationSize, infectionRate, latentPeriod, subclinicalPeriod, infectiousPeriod, interactionsPerDay, numHomes, familiesPerHome, numWorkplaces, workersFraction, false, immunityDuration, transitionLatent, transitionHidden, transitionHome, transitionHospital, recoveryChance, deathChance)
	model.initializeInfected(1000)
	for i, dp := range data {
		if i > 51 {
			break
		}
		// fmt.Println(dp)
		rand.Seed(time.Now().Unix())

		model.step()
		model.step()
		model.step()
		model.step()
		model.step()
		model.step()
		model.step()

		metrics := CalculateMetrics(model.history)
		errorInfections := float64(metrics.ActiveCases) - float64(dp.ActiveCases)
		totalError += math.Pow(errorInfections, 2)
	}

	return totalError
}

func CalculateMetrics(history map[string][]int) Metrics {
	metrics := Metrics{}
	metrics.ActiveCases = 0
	infections, exists := history["IH"]
	if exists && len(infections) > 0 {
		metrics.ActiveCases = infections[len(infections)-1]
	}
	return metrics
}

var months = map[string]time.Month{
	"янв": time.January,
	"фев": time.February,
	"мар": time.March,
	"апр": time.April,
	"мая": time.May,
	"июн": time.June,
	"июл": time.July,
	"авг": time.August,
	"сен": time.September,
	"окт": time.October,
	"ноя": time.November,
	"дек": time.December,
}

func parseDate(dateStr string) (time.Time, error) {
	parts := strings.Split(dateStr, " ")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid date format")
	}

	day, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, err
	}

	month, ok := months[strings.ToLower(parts[1])]
	if !ok {
		return time.Time{}, fmt.Errorf("invalid month abbreviation")
	}

	year, err := strconv.Atoi(parts[2])
	if err != nil {
		return time.Time{}, err
	}

	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC), nil
}

func cleanActiveCases(value string) int {
	value = strings.ReplaceAll(value, "▲", " ")
	value = strings.ReplaceAll(value, "▼", " ")

	value = strings.ReplaceAll(value, " ", "")

	value = strings.ReplaceAll(value, "+", " +")
	value = strings.ReplaceAll(value, "-", " -")

	parts := strings.Split(value, " ")
	if len(parts) == 0 {
		return -1
	}

	num, err := strconv.Atoi(parts[0])
	if err != nil {
		return -1
	}

	return num
}

func read(path string) []DataPoint {
	file, err := os.Open(path)
	if err != nil {
		fmt.Println("Ошибка открытия файла:", err)
		return nil
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ','
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		fmt.Println("Ошибка чтения заголовков:", err)
		return nil
	}

	if len(headers) != 9 {
		fmt.Println("Неверное количество столбцов в заголовке")
		return nil
	}

	var data []DataPoint

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("Ошибка чтения записи:", err)
			return nil
		}

		if len(record) != 9 {
			fmt.Println("Неверное количество полей в записи:", record)
			continue
		}
		var dp DataPoint

		dateStr := record[0]
		date, err := parseDate(dateStr)
		if err != nil {
			fmt.Printf("Ошибка получения данных '%s': %v\n", dateStr, err)
			continue
		}

		dp.Date = date
		dp.ActiveCases = cleanActiveCases(record[8])
		if dp.ActiveCases == -1 {
			dp.ActiveCases = 0
		}

		data = append(data, dp)
	}

	sort.Slice(data, func(i, j int) bool {
		return data[i].Date.Before(data[j].Date)
	})

	return data
}

func cleanNumber(s string) string {
	return strings.ReplaceAll(s, " ", "")
}

func parseInt(s string) int {
	num, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return num
}

func main() {
	rand.Seed(time.Now().Unix())
	populationSize := 10382754
	latentPeriod := 2
	subclinicalPeriod := 2
	infectiousPeriod := 14
	interactionsPerDay := 5
	numHomes := 3808650
	familiesPerHome := 4
	numWorkplaces := 824000
	workersFraction := 0.7

	// swarmSize := 16
	// iterations := 10
	// dim := 8

	// lb := []float64{0.007, 100.0, 0.6, 0.6, 0.6, 0.6, 0.86, 0.012}
	// ub := []float64{0.2, 200.0, 0.9, 0.9, 0.9, 0.9, 0.97, 0.015}
	// data := read("data.csv")

	// for _, r := range data {
	// 	fmt.Printf("%+v\n", r)
	// }
	// _, BestParam := pso(data, swarmSize, iterations, dim, lb, ub)

	// infectionRate := BestParam.infectionRate           // 0.014
	// immunityDuration := BestParam.immunityDuration     // 180
	// transitionLatent := BestParam.transitionLatent     // 0.6
	// transitionHidden := BestParam.transitionHidden     // 0.9
	// transitionHome := BestParam.transitionHome         // 0.9
	// transitionHospital := BestParam.transitionHospital // 0.9
	// recoveryChance := BestParam.recoveryChance         // 0.9
	// deathChance := BestParam.deathChance               // 0.02

	infectionRate := 0.13   //0.13
	immunityDuration := 180 // 180
	transitionLatent := 0.6 // 0.6
	transitionHidden := 0.7
	transitionHome := 0.4     // 0.9
	transitionHospital := 0.6 // 0.9
	recoveryChance := 0.965   // 0.9
	deathChance := 0.01       // 0.02

	model := Model(populationSize, infectionRate, latentPeriod, subclinicalPeriod, infectiousPeriod, interactionsPerDay, numHomes, familiesPerHome, numWorkplaces, workersFraction, false, immunityDuration, transitionLatent, transitionHidden, transitionHome, transitionHospital, recoveryChance, deathChance)
	model.initializeInfected(10000)

	for day := 0; day < 1736; day++ {
		model.step()
	}

	file, err := os.Create("history.txt")
	if err != nil {
		fmt.Println("Ошибка при создании файла:", err)
		return
	}
	defer file.Close()

	for key, values := range model.history {
		_, err := fmt.Fprintf(file, "%s: ", key)
		if err != nil {
			fmt.Println("Ошибка при записи в файл:", err)
			return
		}

		for i, value := range values {
			_, err := fmt.Fprintf(file, "%v", value)
			if err != nil {
				fmt.Println("Ошибка при записи в файл:", err)
				return
			}

			if i < len(values)-1 {
				_, err := fmt.Fprintf(file, ", ")
				if err != nil {
					fmt.Println("Ошибка при записи в файл:", err)
					return
				}
			}
		}
		_, err = fmt.Fprintln(file)
		if err != nil {
			fmt.Println("Ошибка при записи в файл:", err)
			return
		}
	}

	fmt.Println("Данные успешно сохранены в файле history.txt")
}
